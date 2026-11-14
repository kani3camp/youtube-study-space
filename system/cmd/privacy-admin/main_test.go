package main

import (
	"testing"

	"app.modules/core/mybigquery"
	"app.modules/internal/privacyops"
)

func TestValidateDeleteConfirmation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		channelID    string
		confirmation string
		wantErr      bool
	}{
		{name: "matching", channelID: "UC123", confirmation: "UC123", wantErr: false},
		{name: "empty channel", channelID: "", confirmation: "", wantErr: true},
		{name: "empty confirmation", channelID: "UC123", confirmation: "", wantErr: true},
		{name: "mismatch", channelID: "UC123", confirmation: "UC456", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateDeleteConfirmation(tt.channelID, tt.confirmation)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateDeleteConfirmation() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPrimaryInventoryIsEmpty(t *testing.T) {
	t.Parallel()

	empty := inspectOutput{
		Firestore: privacyops.FirestoreInventory{
			Collections: map[string]int64{"users": 0, "live-chat-history": 0},
		},
		BigQuery: mybigquery.UserDataInventory{},
	}
	if !primaryInventoryIsEmpty(empty) {
		t.Fatal("primaryInventoryIsEmpty() = false for empty inventory")
	}

	withFirestoreData := empty
	withFirestoreData.Firestore.Collections = map[string]int64{"users": 1}
	if primaryInventoryIsEmpty(withFirestoreData) {
		t.Fatal("primaryInventoryIsEmpty() = true with Firestore data")
	}

	withMyPageData := empty
	withMyPageData.Firestore.Collections = map[string]int64{"users": 0}
	withMyPageData.Firestore.MyPage.ChannelOwnerDocument = 1
	if primaryInventoryIsEmpty(withMyPageData) {
		t.Fatal("primaryInventoryIsEmpty() = true with MyPage data")
	}

	withBigQueryData := empty
	withBigQueryData.Firestore.Collections = map[string]int64{"users": 0}
	withBigQueryData.BigQuery.LiveChatHistoryRows = 1
	if primaryInventoryIsEmpty(withBigQueryData) {
		t.Fatal("primaryInventoryIsEmpty() = true with BigQuery data")
	}
}
