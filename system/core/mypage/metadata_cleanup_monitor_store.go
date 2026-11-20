package mypage

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// このサーバー専用単一文書には、既存の既定拒否クライアントルールが適用される。
// 認証情報・IAM・保持設定・スケジューラーは構築しない。
type FirestoreMetadataCleanupMonitor struct {
	Client    *firestore.Client
	ProjectID string
}

func (s *FirestoreMetadataCleanupMonitor) valid() bool {
	return (&FirestoreMetadataCleanupStore{Client: s.Client, ProjectID: s.ProjectID}).valid()
}

func (s *FirestoreMetadataCleanupMonitor) ref() *firestore.DocumentRef {
	return s.Client.Collection("mypage-maintenance").Doc("metadata-cleanup")
}

func decodeMetadataCleanupMonitoring(doc *firestore.DocumentSnapshot, err error) (MetadataCleanupMonitoringState, error) {
	if status.Code(err) == codes.NotFound {
		return MetadataCleanupMonitoringState{}, nil
	}
	if err != nil {
		return MetadataCleanupMonitoringState{}, fmt.Errorf("read cleanup monitoring: %w", err)
	}
	var state MetadataCleanupMonitoringState
	if err := doc.DataTo(&state); err != nil {
		return state, fmt.Errorf("decode cleanup monitoring: %w", err)
	}
	if !validMetadataCleanupMonitoringState(state) {
		return state, ErrMetadataCleanupIncomplete
	}
	return state, nil
}

func (s *FirestoreMetadataCleanupMonitor) Load(ctx context.Context) (MetadataCleanupMonitoringState, error) {
	if !s.valid() {
		return MetadataCleanupMonitoringState{}, ErrMetadataCleanupIncomplete
	}
	return decodeMetadataCleanupMonitoring(s.ref().Get(ctx))
}

func (s *FirestoreMetadataCleanupMonitor) Record(ctx context.Context, next MetadataCleanupObservation) error {
	if !s.valid() || !validMetadataCleanupMonitoringObservation(next) {
		return ErrMetadataCleanupIncomplete
	}
	if err := s.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		state, err := decodeMetadataCleanupMonitoring(tx.Get(s.ref()))
		if err != nil {
			return err
		}
		state = updateMetadataCleanupMonitoring(state, next)
		if err := tx.Set(s.ref(), state); err != nil {
			return fmt.Errorf("write cleanup monitoring: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("commit cleanup monitoring: %w", err)
	}
	return nil
}

// NewFirestoreMetadataCleanupRunner は削除処理と監視状態ストアを、指定された同一プロジェクト・クライアントに紐づける。
// 構築時には処理を開始せず、スケジューラーや期限対応の出力先は呼び出し元が管理する。
func NewFirestoreMetadataCleanupRunner(client *firestore.Client, projectID string, signals MetadataCleanupHealthRecorder, now func() time.Time, batchSize, maxPages int) (*MetadataCleanupRunner, error) {
	store := &FirestoreMetadataCleanupStore{Client: client, ProjectID: projectID}
	if !store.valid() || signals == nil || now == nil || batchSize < 1 || batchSize > 1000 || maxPages < 1 || maxPages > 1000 {
		return nil, ErrMetadataCleanupIncomplete
	}
	return &MetadataCleanupRunner{
		Store: store, Monitor: &FirestoreMetadataCleanupMonitor{Client: client, ProjectID: projectID},
		Signals: signals, Now: now, BatchSize: batchSize, MaxPages: maxPages,
	}, nil
}

// ACK は期限対応の出力先が違反シグナルを受理したことを示す（運用担当者の確認完了までは意味しない）。
// 想定時刻を比較してから消去することで、並行して記録された新しい期限違反を上書きせずに残す。
func (s *FirestoreMetadataCleanupMonitor) AcknowledgeRetention(ctx context.Context, startedAt time.Time) error {
	if !s.valid() || startedAt.IsZero() {
		return ErrMetadataCleanupIncomplete
	}
	startedAt = startedAt.UTC().Truncate(time.Microsecond)
	if err := s.Client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		state, err := decodeMetadataCleanupMonitoring(tx.Get(s.ref()))
		if err != nil {
			return err
		}
		if !state.PendingRetentionStartedAt.Equal(startedAt) {
			return nil
		}
		state.PendingRetentionStartedAt = time.Time{}
		if err := tx.Set(s.ref(), state); err != nil {
			return fmt.Errorf("acknowledge cleanup retention signal: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("commit cleanup retention acknowledgement: %w", err)
	}
	return nil
}
