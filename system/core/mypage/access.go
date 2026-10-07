package mypage

import (
	"context"
	"errors"

	"cloud.google.com/go/firestore"

	"app.modules/core/serviceaccess"
)

// accessGateError distinguishes an authorization-store outage from an ordinary
// metadata-provider outage. It must never become a stale/partial 200 response.
type accessGateError struct{ cause error }

func (e *accessGateError) Error() string { return e.cause.Error() }
func (e *accessGateError) Unwrap() error { return e.cause }

func accessVerdict(value serviceaccess.Snapshot, err error, expected string) (string, error) {
	if err == nil {
		err = value.Allowed()
	}
	code := "TEMPORARY_UNAVAILABLE"
	switch {
	case errors.Is(err, serviceaccess.ErrDeletion):
		code = "DATA_DELETION_IN_PROGRESS"
	case errors.Is(err, serviceaccess.ErrRestricted):
		code = "SERVICE_ACCESS_RESTRICTED"
	case err == nil && (expected == "" || expected == value.Checkpoint()):
		return value.Checkpoint(), nil
	}
	return "", &accessGateError{cause: apiError(code)}
}

func readAccess(ctx context.Context, reader serviceaccess.Reader, uid, expected string) (string, error) {
	if reader == nil {
		return accessVerdict(serviceaccess.Snapshot{}, serviceaccess.ErrUnavailable, expected)
	}
	value, err := reader.Read(ctx, uid)
	return accessVerdict(value, err, expected)
}

func (s *FirestoreAuthStore) transactionAccess(tx *firestore.Transaction, uid, expected string) error {
	value, err := (&serviceaccess.FirestoreStore{Client: s.Client}).ReadTransaction(tx, uid)
	_, err = accessVerdict(value, err, expected)
	return err
}

func (s *FirestoreAuthStore) recheckMetadataAccess(ctx context.Context, uid string, account *WebAccount, expected string) error {
	checkpoint, err := readAccess(ctx, &serviceaccess.FirestoreStore{Client: s.Client}, uid, expected)
	if err == nil {
		account.AccessCheckpoint = checkpoint
	}
	return err
}
