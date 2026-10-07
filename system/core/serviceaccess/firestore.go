package serviceaccess

import (
	"context"
	"errors"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FirestoreStore struct{ Client *firestore.Client }

func (s *FirestoreStore) ref(channel string) (*firestore.DocumentRef, error) {
	if s == nil || s.Client == nil || !ValidChannel(channel) {
		return nil, ErrUnavailable
	}
	return s.Client.Collection(Collection).Doc(channel), nil
}

func decode(doc *firestore.DocumentSnapshot, err error) (Snapshot, error) {
	if status.Code(err) == codes.NotFound {
		return Snapshot{}, nil
	}
	if err != nil || doc == nil {
		return Snapshot{}, ErrUnavailable
	}
	data := doc.Data()
	if len(data) != 4 {
		return Snapshot{}, ErrUnavailable
	}
	for _, key := range []string{"generation", "updatedAt", "moderation", "privacyDeletion"} {
		if _, ok := data[key]; !ok {
			return Snapshot{}, ErrUnavailable
		}
	}
	if _, ok := data["generation"].(int64); !ok {
		return Snapshot{}, ErrUnavailable
	}
	if _, ok := data["updatedAt"].(time.Time); !ok {
		return Snapshot{}, ErrUnavailable
	}
	for _, key := range []string{"moderation", "privacyDeletion"} {
		r, ok := data[key].(map[string]interface{})
		if !ok {
			return Snapshot{}, ErrUnavailable
		}
		if _, ok := r["active"].(bool); !ok {
			return Snapshot{}, ErrUnavailable
		}
		for field, value := range r {
			switch field {
			case "active":
			case "since":
				if _, ok := value.(time.Time); !ok {
					return Snapshot{}, ErrUnavailable
				}
			case "reasonCode", "actionRef", "requestRef":
				if _, ok := value.(string); !ok {
					return Snapshot{}, ErrUnavailable
				}
			default:
				return Snapshot{}, ErrUnavailable
			}
		}
	}
	var control Control
	if doc.DataTo(&control) != nil {
		return Snapshot{}, ErrUnavailable
	}
	value := Snapshot{Control: control, Exists: true, Revision: doc.UpdateTime}
	if value.Validate() != nil {
		return Snapshot{}, ErrUnavailable
	}
	return value, nil
}

func (s *FirestoreStore) Read(ctx context.Context, channel string) (Snapshot, error) {
	ref, err := s.ref(channel)
	if err != nil {
		return Snapshot{}, err
	}
	doc, err := ref.Get(ctx)
	return decode(doc, err)
}

// ReadTransaction must precede every write in a retryable transaction. A
// concurrent control change participates in Firestore conflict detection.
func (s *FirestoreStore) ReadTransaction(tx *firestore.Transaction, channel string) (Snapshot, error) {
	ref, err := s.ref(channel)
	if err != nil || tx == nil {
		return Snapshot{}, ErrUnavailable
	}
	doc, err := tx.Get(ref)
	return decode(doc, err)
}

func (s *FirestoreStore) Change(ctx context.Context, channel string, change Change, now time.Time) (Snapshot, error) {
	if _, _, err := Transition(Snapshot{}, change, now); err != nil {
		return Snapshot{}, err
	}
	ref, err := s.ref(channel)
	if err != nil {
		return Snapshot{}, err
	}
	err = s.Client.RunTransaction(ctx, func(_ context.Context, tx *firestore.Transaction) error {
		old, err := s.ReadTransaction(tx, channel)
		if err != nil {
			return err
		}
		next, changed, err := Transition(old, change, now)
		if err != nil || !changed {
			return err
		}
		if err := tx.Set(ref, next); err != nil {
			return ErrUnavailable
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrInvalidChange) {
			return Snapshot{}, ErrInvalidChange
		}
		return Snapshot{}, ErrUnavailable
	}
	return s.Read(ctx, channel)
}
