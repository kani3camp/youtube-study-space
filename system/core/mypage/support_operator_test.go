package mypage

import (
	"context"
	"errors"
	"testing"
)

func TestOperatorDefaultDenied(t *testing.T) {
	s := &FirestoreSupportOperator{}
	err := s.Reply(context.Background(), ReplyOperation{})
	if !errors.Is(err, ErrOperatorDenied) {
		t.Fatalf("missing authority accepted: %v", err)
	}
	if err := s.Complete(context.Background(), CompletionOperation{}); !errors.Is(err, ErrOperatorDenied) {
		t.Fatalf("missing authority accepted: %v", err)
	}
}
