package supportdelete

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SyntheticCatalogは独立して用意したテストデータから取り、運用入力は使わない。
// 既存の静的なソース一覧は実行時の実環境一覧ではない。
// 準備では接続と対応を検査し、証拠の由来や事後条件は検査しない。
type SyntheticCatalog struct {
	Selector      Selector
	Manifest      Manifest
	Registrations []AdapterRegistration
}

// StoreFactoryは合成接続の完全な検証後だけ呼び出す。
// クライアントのライフサイクルは注入元が管理し、このパッケージでは作らない。
type StoreFactory func(context.Context, Target) (Store, error)

// PreparedDeletionは、エンジン・アダプター設定を変更可能な形で公開しない。
// 1つの対象指定・マニフェストに固定し、明示的な復旧時だけ所有者を移す。
type PreparedDeletion struct {
	mu       sync.Mutex
	engine   Engine
	selector Selector
	owner    string
}

func PrepareDeletion(ctx context.Context, selector Selector, owner string, catalog SyntheticCatalog, factory StoreFactory, clock func() time.Time) (*PreparedDeletion, error) {
	if ctx == nil || catalog.Selector != selector {
		return nil, ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("prepare deletion interrupted: %w", err)
	}
	registry, err := NewRegistry(selector, catalog.Manifest, catalog.Registrations)
	if err != nil {
		return nil, fmt.Errorf("prepare deletion registry: %w", err)
	}
	if !ValidRef(owner) || factory == nil || clock == nil || clock().IsZero() {
		return nil, ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("prepare deletion interrupted: %w", err)
	}
	store, err := factory(ctx, selector.Target)
	if err != nil {
		return nil, fmt.Errorf("prepare deletion store: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("prepare deletion interrupted: %w", err)
	}
	if nilDependency(store) {
		return nil, ErrUnavailable
	}
	return &PreparedDeletion{
		engine:   Engine{Store: store, Effects: registry, Manifest: catalog.Manifest, Clock: clock},
		selector: selector, owner: owner,
	}, nil
}

func (p *PreparedDeletion) blocked() Outcome {
	mode := "unknown"
	if p != nil {
		mode = p.engine.mode()
	}
	return Outcome{Code: "WORKFLOW_BLOCKED", Stage: "blocked", Mode: mode, Offline: true, ActualExecution: false}
}

func (p *PreparedDeletion) Run(ctx context.Context) (Outcome, error) {
	if p == nil || ctx == nil {
		return p.blocked(), ErrUnavailable
	}
	if !p.mu.TryLock() {
		return p.blocked(), ErrBusy
	}
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return p.blocked(), fmt.Errorf("deletion interrupted: %w", err)
	}
	return p.engine.Run(ctx, p.selector, p.owner)
}

// 呼び出し元の復旧案は、旧worker停止の証拠にはならない。
// Storeはその追跡情報・永続リビジョンと世代・時刻を独立して検証する必要がある。
// ローカルロックは、他のプロセスや全実行系の処理完了の証拠にはしない。
func (p *PreparedDeletion) Recover(ctx context.Context, recovery Recovery) error {
	if p == nil || ctx == nil || nilDependency(p.engine.Store) || p.engine.Clock == nil {
		return ErrUnavailable
	}
	if !p.mu.TryLock() {
		return ErrBusy
	}
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("deletion recovery interrupted: %w", err)
	}
	if recovery.Selector != p.selector || recovery.ManifestRef != p.engine.Manifest.Ref ||
		recovery.PreviousOwnerRef != p.owner || !ValidRef(recovery.NewOwnerRef) || recovery.NewOwnerRef == p.owner {
		return ErrConflict
	}
	if err := p.engine.Store.Recover(ctx, recovery, p.engine.Clock()); err != nil {
		return fmt.Errorf("recover prepared deletion: %w", err)
	}
	p.owner = recovery.NewOwnerRef
	return nil
}
