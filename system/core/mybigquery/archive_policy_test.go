package mybigquery

import (
	"testing"

	"app.modules/core/repository"
)

func TestShouldArchiveCollectionToBigQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		collection string
		want       bool
	}{
		{
			name:       "raw live chat is never archived",
			collection: repository.LiveChatHistory,
			want:       false,
		},
		{
			name:       "user activities remain archived",
			collection: repository.UserActivities,
			want:       true,
		},
		{
			name:       "order history remains archived",
			collection: repository.OrderHistory,
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldArchiveCollectionToBigQuery(tt.collection); got != tt.want {
				t.Fatalf("shouldArchiveCollectionToBigQuery(%q) = %t, want %t", tt.collection, got, tt.want)
			}
		})
	}
}
