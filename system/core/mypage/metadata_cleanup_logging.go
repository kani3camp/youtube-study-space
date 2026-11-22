package mypage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"golang.org/x/oauth2"
)

const (
	CloudLoggingEntriesWriteEndpoint = "https://logging.googleapis.com/v2/entries:write"
	// 健全性シグナル専用のログ名。アラートやログ保持はこの名前だけを条件にする。
	MetadataCleanupHealthLogID = "mypage-metadata-cleanup-health"
	// Cloud Logging の書き込みに必要な最小スコープ。
	CloudLoggingWriteScope = "https://www.googleapis.com/auth/logging.write"
)

var (
	cloudLoggingProjectID = regexp.MustCompile(`^[a-z][-a-z0-9]{4,28}[a-z0-9]$`)
	cloudLoggingLocation  = regexp.MustCompile(`^[a-z]+-[a-z]+[0-9]+$`)
	cloudRunJobName       = regexp.MustCompile(`^[a-z]([-a-z0-9]{0,61}[a-z0-9])?$`)
)

// CloudLoggingMetadataCleanupHealthRecorder は健全性シグナルを Cloud Logging へ 1 件の構造化ログとして書き込む。
// 標準出力は書き込み期限を設定できないため、期限を HTTP リクエストの ctx で適用できるこの出力先を使う。
// ログには固定のログ名・Cloud Run Job リソース・checkedAt/alert/reason だけを含め、ID や元のエラーは含めない。
// HTTP 2xx を受け取ったときだけ受理とみなし、ランナーはその後にだけ保持期限違反へ ACK する。
type CloudLoggingMetadataCleanupHealthRecorder struct {
	ProjectID string
	Location  string
	JobName   string
	Tokens    oauth2.TokenSource
	// client と endpoint はパッケージ内テスト専用の差し替え口で、外部からトークンの送信先を変えられないよう非公開にする。
	// 未設定なら公式エンドポイントを使う。client を差し替えてもリダイレクト拒否とタイムアウト上限は常に適用する。
	client   *http.Client
	endpoint string
}

type cloudLoggingWriteRequest struct {
	Entries []cloudLoggingEntry `json:"entries"`
}

type cloudLoggingEntry struct {
	LogName     string                           `json:"logName"`
	Resource    cloudLoggingResource             `json:"resource"`
	Severity    string                           `json:"severity"`
	JSONPayload MetadataCleanupHealthObservation `json:"jsonPayload"`
}

type cloudLoggingResource struct {
	Type   string            `json:"type"`
	Labels map[string]string `json:"labels"`
}

var errCloudLoggingToken = errors.New("cloud logging token unavailable")

// ValidateResource はログ名とリソースラベルに使うプロジェクト・リージョン・ジョブ名を検査する。
// 呼び出し元は削除処理を始める前にこれを確認し、シグナルを書けない構成で削除を開始しない。
func (r *CloudLoggingMetadataCleanupHealthRecorder) ValidateResource() error {
	if !cloudLoggingProjectID.MatchString(r.ProjectID) || !cloudLoggingLocation.MatchString(r.Location) || !cloudRunJobName.MatchString(r.JobName) {
		return ErrMetadataCleanupIncomplete
	}
	return nil
}

func (r *CloudLoggingMetadataCleanupHealthRecorder) RecordHealth(ctx context.Context, value MetadataCleanupHealthObservation) error {
	if !validMetadataCleanupHealthObservation(value) || r.Tokens == nil || r.ValidateResource() != nil {
		return ErrMetadataCleanupIncomplete
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cleanup health log canceled: %w", err)
	}
	severity := "INFO"
	if value.Alert {
		severity = "WARNING"
	}
	value.CheckedAt = value.CheckedAt.UTC()
	body, err := json.Marshal(cloudLoggingWriteRequest{Entries: []cloudLoggingEntry{{
		LogName: "projects/" + r.ProjectID + "/logs/" + MetadataCleanupHealthLogID,
		Resource: cloudLoggingResource{Type: "cloud_run_job", Labels: map[string]string{
			"job_name": r.JobName, "location": r.Location, "project_id": r.ProjectID,
		}},
		Severity:    severity,
		JSONPayload: value,
	}}})
	if err != nil {
		return fmt.Errorf("encode cleanup health log: %w", err)
	}
	token, err := cloudLoggingToken(ctx, r.Tokens)
	if err != nil {
		return err
	}
	endpoint := r.endpoint
	if endpoint == "" {
		endpoint = CloudLoggingEntriesWriteEndpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build cleanup health log request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	token.SetAuthHeader(req)
	bounded := http.Client{Timeout: 5 * time.Second}
	if r.client != nil {
		bounded = *r.client
	}
	if bounded.Timeout <= 0 || bounded.Timeout > 5*time.Second {
		bounded.Timeout = 5 * time.Second
	}
	// Bearer トークンをリダイレクト先へ転送しない。
	bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := bounded.Do(req)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("cleanup health log canceled: %w", ctxErr)
		}
		return fmt.Errorf("send cleanup health log: %w", err)
	}
	// 応答本文は読み捨て、エラーには含めない（ステータスコードだけを返す）。
	_, drainErr := io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	closeErr := resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("cleanup health log rejected: status %d", resp.StatusCode)
	}
	// 受理の確認が不完全なら未受理として扱い、ACK せずに再通知させる。
	if err := errors.Join(drainErr, closeErr); err != nil {
		return fmt.Errorf("read cleanup health log response: %w", err)
	}
	return nil
}

// oauth2.TokenSource は ctx を受け取らないため、取得を別 goroutine で行い ctx の期限で待機を打ち切る。
// トークン取得は副作用のない読み取りなので、打ち切った取得が後から完了しても安全に捨てられる。
// 取得失敗の詳細（認証サーバーの応答本文を含みうる）は返さない。
func cloudLoggingToken(ctx context.Context, tokens oauth2.TokenSource) (*oauth2.Token, error) {
	type result struct {
		token *oauth2.Token
		err   error
	}
	done := make(chan result, 1)
	go func() {
		token, err := tokens.Token()
		done <- result{token, err}
	}()
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("cleanup health log canceled: %w", ctx.Err())
	case got := <-done:
		if got.err != nil || got.token == nil || got.token.AccessToken == "" {
			return nil, errCloudLoggingToken
		}
		return got.token, nil
	}
}
