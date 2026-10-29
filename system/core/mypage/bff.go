package mypage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"app.modules/core/serviceaccess"
)

type (
	metadataFlight struct {
		done     chan struct{}
		response Response
		err      error
		terminal bool
	}
	cacheKey        struct{ environment, uid string }
	cachedAggregate struct {
		response       Response
		accountVersion string
	}
	MetadataRefresher interface {
		Refresh(context.Context, string, WebAccount) (WebAccount, error)
	}
)

// BFFはプロセス内メモリだけを使う。最新のアクセス状態を読み、キャッシュ参照・
// 独立した共有処理・結果の公開・レスポンス返却の各段階を保護する。inactiveの許可結果はキャッシュしない。
type BFF struct {
	Access      serviceaccess.Reader
	Reader      SnapshotReader
	Environment string
	Now         func() time.Time
	Metadata    MetadataRefresher
	mu          sync.Mutex
	cache       map[cacheKey]cachedAggregate
	flights     map[string]*metadataFlight
}

func (b *BFF) cacheHit(key cacheKey, now time.Time, account WebAccount) (Response, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	value, ok := b.cache[key]
	if !ok {
		return Response{}, false
	}
	asOf := value.response.GeneratedAt
	if value.accountVersion != metadataAccountVersion(account) || (accountSection(account, now).Data == nil && value.response.Account.Data != nil) || now.Before(asOf) || now.Sub(asOf) >= 30*time.Second || !StatisticsWindows(now).Today.Equal(StatisticsWindows(asOf).Today) {
		delete(b.cache, key)
		return Response{}, false
	}
	return value.response, true
}

func accountSection(account WebAccount, now time.Time) Section[Account] {
	if account.DisplayName == "" {
		// 他のプロセスがfetchedAtを消去した後も、空のメタデータは表示停止状態として扱う。
		return missingSection[Account](MetadataTooOld)
	}
	age := now.Sub(account.MetadataFetchedAt)
	if account.MetadataFetchedAt.IsZero() || age >= 30*24*time.Hour || age < 0 {
		return missingSection[Account](MetadataTooOld)
	}
	data := Account{DisplayName: account.DisplayName, Handle: account.Handle, AvatarURL: account.AvatarURL}
	if age >= 24*time.Hour {
		return Section[Account]{Availability: Partial, ReasonCode: ptr(MetadataRefreshFailed), Data: &data}
	}
	return availableSection(data)
}

func (b *BFF) Get(ctx context.Context, uid string, account WebAccount) (result Response, resultErr error) {
	checkpoint, err := readAccess(ctx, b.Access, uid, account.AccessCheckpoint)
	if err != nil {
		b.Invalidate(uid)
		return Response{}, err
	}
	account.AccessCheckpoint = checkpoint
	defer func() {
		if resultErr == nil {
			if _, err := readAccess(ctx, b.Access, uid, checkpoint); err != nil {
				b.Invalidate(uid)
				result, resultErr = Response{}, err
			}
		}
	}()
	key := cacheKey{b.Environment, uid}
	now := b.Now().UTC()
	if value, ok := b.cacheHit(key, now, account); ok {
		return value, nil
	}
	// HTTPの制限時間は認可前から進み、ブラウザにも期限がある。
	// 共有処理の結果返却とレスポンスのシリアライズに余裕を残し、
	// キャンセルを切り離しても消費済みの制限時間をリセットしない。
	workDeadline := time.Now().Add(8 * time.Second)
	waitCtx := ctx
	if outerDeadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(outerDeadline)
		if remaining <= 0 {
			return Response{}, apiError("TEMPORARY_UNAVAILABLE")
		}
		reserve := min(time.Second, remaining/4)
		waitDeadline := outerDeadline.Add(-reserve)
		workDeadline = minDeadline(workDeadline, waitDeadline.Add(-reserve))
		var stop context.CancelFunc
		waitCtx, stop = context.WithDeadline(ctx, waitDeadline)
		defer stop()
	}
	// 最初の呼び出し元の切断で、別の呼び出し元と共有するリクエストを中断しない。
	// 共有操作自体にも独立した制限時間を設ける。
	flightKey := b.Environment + "\x00" + uid + "\x00" + metadataAccountVersion(account)
	flight, leader := b.acquireFlight(flightKey)
	if flight == nil {
		return b.failedSnapshot(account)
	}
	if leader {
		go func() {
			response := Response{}
			flightErr := apiError("TEMPORARY_UNAVAILABLE")
			defer func() {
				b.mu.Lock()
				flight.response, flight.err = response, flightErr
				delete(b.flights, flightKey)
				close(flight.done)
				b.mu.Unlock()
			}()
			response, flightErr = func() (result Response, resultErr error) {
				// 共有処理側のコピーだけを更新し、呼び出し元の早期の代替応答と、
				// 独立した共有処理のメタデータ更新が競合しないようにする。
				account := account
				workCtx, cancel := context.WithDeadline(context.WithoutCancel(ctx), workDeadline)
				defer cancel()
				if _, err := readAccess(workCtx, b.Access, uid, checkpoint); err != nil {
					return Response{}, err
				}
				defer func() {
					if resultErr == nil {
						if _, err := readAccess(workCtx, b.Access, uid, checkpoint); err != nil {
							result, resultErr = Response{}, err
						}
					}
				}()
				if value, ok := b.cacheHit(key, b.Now().UTC(), account); ok {
					return value, nil
				}
				if b.Metadata != nil && (account.MetadataFetchedAt.IsZero() || b.Now().Sub(account.MetadataFetchedAt) >= 24*time.Hour) {
					metadataCtx, stop := context.WithTimeout(workCtx, time.Second)
					if refreshed, err := b.Metadata.Refresh(metadataCtx, uid, account); err == nil {
						account = refreshed
					} else if metadataGateError(err) {
						stop()
						gateErr := fmt.Errorf("metadata access gate: %w", err)
						b.mu.Lock()
						flight.err = gateErr
						b.mu.Unlock()
						return Response{}, gateErr
					} else if errors.Is(err, ErrPublicChannelMissing) {
						// 永続データの消去に失敗しても、不在が証明されたデータの表示は停止する。
						account.DisplayName = ""
						account.Handle, account.AvatarURL = nil, nil
						account.MetadataFetchedAt = time.Time{}
					}
					stop()
				}
				section := accountSection(account, b.Now().UTC())
				b.mu.Lock()
				flight.terminal = section.ReasonCode != nil && *section.ReasonCode == MetadataTooOld
				b.mu.Unlock()

				snapshot, err := b.Reader.Read(workCtx, uid)
				if err != nil {
					return b.failedSnapshot(account)
				}
				response, err := Aggregate(snapshot, accountSection(account, b.Now().UTC()))
				if err != nil {
					return b.failedSnapshot(account)
				}
				if _, err := readAccess(workCtx, b.Access, uid, checkpoint); err != nil {
					return Response{}, err
				}
				b.mu.Lock()
				if b.cache == nil {
					b.cache = make(map[cacheKey]cachedAggregate)
				}
				// 攻撃者が指定できるuidの種類数を制限し、上限時は1件を追い出す。
				if len(b.cache) >= 1024 {
					for victim := range b.cache {
						delete(b.cache, victim)
						break
					}
				}
				b.cache[key] = cachedAggregate{response: response, accountVersion: metadataAccountVersion(account)}
				b.mu.Unlock()
				return response, nil
			}()
		}()
	}
	select {
	case <-waitCtx.Done():
		// 後から合流する呼び出し元の制限時間は、最初の呼び出し元より短い場合がある。
		// 既知の表示停止アカウント情報をその期限前に返し、
		// 上限付き共有処理は他の呼び出し元のために継続する。
		if errors.Is(waitCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
			b.mu.Lock()
			gateErr, terminal := flight.err, flight.terminal
			b.mu.Unlock()
			if metadataGateError(gateErr) {
				return Response{}, gateErr
			}
			if terminal {
				return b.failedSnapshot(WebAccount{})
			}
			return b.failedSnapshot(account)
		}
		return Response{}, apiError("TEMPORARY_UNAVAILABLE")
	case <-flight.done:
		if flight.err != nil {
			if metadataGateError(flight.err) {
				return Response{}, flight.err
			}
			return Response{}, apiError("TEMPORARY_UNAVAILABLE")
		}
		return flight.response, nil
	}
}

// 登録と完了は同じロックで行い、合流する全呼び出し元に同じ認証済みメタデータ判定を渡す。
// 登録件数に上限を設け、コールバック完了時の結果公開とスロット削除はアトミックに行う。
func (b *BFF) acquireFlight(key string) (*metadataFlight, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if flight := b.flights[key]; flight != nil {
		return flight, false
	}
	if len(b.flights) >= 1024 {
		return nil, false
	}
	if b.flights == nil {
		b.flights = make(map[string]*metadataFlight)
	}
	flight := &metadataFlight{done: make(chan struct{})}
	b.flights[key] = flight
	return flight, true
}

func minDeadline(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

// 作業を読めなくても、既知のメタデータ表示停止状態を維持する。
// 作業スナップショットは作り出さず、全作業項目を取得不能にする。0や未登録にはせず、失敗応答はキャッシュしない。
// 既存の通信仕様により、ブラウザはアカウント情報を捨て、以前の作業を古い値として保持できる。
func (b *BFF) failedSnapshot(account WebAccount) (Response, error) {
	now := b.Now().UTC()
	section := accountSection(account, now)
	if section.ReasonCode == nil || *section.ReasonCode != MetadataTooOld {
		return Response{}, apiError("TEMPORARY_UNAVAILABLE")
	}
	windows := StatisticsWindows(now)
	days := make([]Day, 0, 7)
	for i := range 7 {
		days = append(days, Day{Date: windows.Recent.AddDate(0, 0, i).Format("2006-01-02"), Metric: missingMetric(SourceUnavailable)})
	}
	return Response{
		GeneratedAt: now, Timezone: "Asia/Tokyo", Partial: true,
		Current: missingSection[Current](SourceUnavailable), Summary: missingSection[Summary](SourceUnavailable),
		Recent7Days: RecentSection{Availability: Unavailable, ReasonCode: ptr(SourceUnavailable), Data: days}, Account: section,
	}, nil
}

// Invalidateはサーバー側のアカウント削除・同意処理で利用できる。
// 各リクエストで必要な検証を代替するものではない。
func (b *BFF) Invalidate(uid string) {
	b.mu.Lock()
	delete(b.cache, cacheKey{b.Environment, uid})
	b.mu.Unlock()
}

func metadataGateError(err error) bool {
	var gate *accessGateError
	if errors.As(err, &gate) {
		return true
	}
	switch errorCode(err) {
	case "AUTH_REQUIRED", "WEB_ACCOUNT_REQUIRED", "PRIVACY_RECONSENT_REQUIRED", "SERVICE_ACCESS_RESTRICTED", "DATA_DELETION_IN_PROGRESS":
		return true
	}
	return false
}

func metadataAccountVersion(account WebAccount) string {
	present := "empty"
	if account.DisplayName != "" && !account.MetadataFetchedAt.IsZero() {
		present = "present"
	}
	return account.AccessCheckpoint + ":" + account.Revision.UTC().Format(time.RFC3339Nano) + ":" + present
}
