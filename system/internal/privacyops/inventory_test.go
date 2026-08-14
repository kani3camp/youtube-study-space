package privacyops

import (
	"context"
	"testing"
)

func TestInspectFirestoreRejectsEmptyChannelID(t *testing.T) {
	t.Parallel()

	_, err := InspectFirestore(context.Background(), nil, "   ")
	if err == nil {
		t.Fatal("InspectFirestore() error = nil, want error")
	}
}
