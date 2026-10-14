package mypage

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// The existing default-deny client rules cover this server-only singleton.
// No credentials, IAM, retention configuration or scheduler are provisioned.
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

// NewFirestoreMetadataCleanupRunner binds account cleanup and durable monitoring
// to the same explicitly selected existing project/client. It starts no work;
// the caller owns the approved runtime, schedule and deadline-capable signal sink.
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
