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

// NewFirestoreMetadataCleanupRunnerは、アカウント削除処理と永続監視を、
// 明示して選んだ同じ既存プロジェクト・クライアントに結び付ける。構築時に処理は開始しない。
// 承認済みランタイム・スケジュール・期限対応の監視出力先は呼び出し元が管理する。
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

// ACKは期限対応の監視出力先が受理したことを示し、運用担当者の受領は意味しない。
// 比較後の消去で、並行して発生した新しいイベントを保持する。
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
