package main

import (
	"app.modules/core/workspaceapp"
	"github.com/stretchr/testify/assert"
	"testing"
)

func newWorkspaceApp() workspaceapp.WorkspaceApp {
	return workspaceapp.WorkspaceApp{}
}

func assertProcessedUserInitialState(t *testing.T, app workspaceapp.WorkspaceApp) {
	t.Helper()

	assert.Equal(t, "", app.ProcessedUserId)
	assert.Equal(t, "", app.ProcessedUserDisplayName)
	assert.Equal(t, "", app.ProcessedUserProfileImageUrl)
	assert.Equal(t, false, app.ProcessedUserIsModeratorOrOwner)
	assert.Equal(t, false, app.ProcessedUserIsMember)
}

func TestSetProcessedUser(t *testing.T) {
	tests := []struct {
		name              string
		isChatModerator   bool
		isChatOwner       bool
		isChatMember      bool
		wantModeratorRole bool
	}{
		{
			name:              "all_false",
			wantModeratorRole: false,
		},
		{
			name:              "moderator_only",
			isChatModerator:   true,
			wantModeratorRole: true,
		},
		{
			name:              "owner_only",
			isChatOwner:       true,
			wantModeratorRole: true,
		},
		{
			name:              "member_only",
			isChatMember:      true,
			wantModeratorRole: false,
		},
		{
			name:              "moderator_and_owner",
			isChatModerator:   true,
			isChatOwner:       true,
			wantModeratorRole: true,
		},
		{
			name:              "moderator_and_member",
			isChatModerator:   true,
			isChatMember:      true,
			wantModeratorRole: true,
		},
		{
			name:              "owner_and_member",
			isChatOwner:       true,
			isChatMember:      true,
			wantModeratorRole: true,
		},
		{
			name:              "moderator_owner_and_member",
			isChatModerator:   true,
			isChatOwner:       true,
			isChatMember:      true,
			wantModeratorRole: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newWorkspaceApp()
			assertProcessedUserInitialState(t, app)

			const userID = "user1-id"
			const userDisplayName = "user1-display-name"
			const userProfileImageURL = "https://example.com/user1-profile-image"

			app.SetProcessedUser(
				userID,
				userDisplayName,
				userProfileImageURL,
				tt.isChatModerator,
				tt.isChatOwner,
				tt.isChatMember,
			)

			assert.Equal(t, userID, app.ProcessedUserId)
			assert.Equal(t, userDisplayName, app.ProcessedUserDisplayName)
			assert.Equal(t, userProfileImageURL, app.ProcessedUserProfileImageUrl)
			assert.Equal(t, tt.wantModeratorRole, app.ProcessedUserIsModeratorOrOwner)
			assert.Equal(t, tt.isChatMember, app.ProcessedUserIsMember)
		})
	}
}

func TestCalculateRetryIntervalSec(t *testing.T) {
	tests := []struct {
		name                string
		numContinuousFailed int
		want                float64
	}{
		{
			name:                "zero_failures",
			numContinuousFailed: 0,
			want:                1,
		},
		{
			name:                "one_failure",
			numContinuousFailed: 1,
			want:                1.2,
		},
		{
			name:                "two_failures",
			numContinuousFailed: 2,
			want:                1.44,
		},
		{
			name:                "three_failures",
			numContinuousFailed: 3,
			want:                1.728,
		},
		{
			name:                "four_failures",
			numContinuousFailed: 4,
			want:                2.0736,
		},
		{
			name:                "five_failures",
			numContinuousFailed: 5,
			want:                2.48832,
		},
		{
			name:                "ten_failures",
			numContinuousFailed: 10,
			want:                6.191736422,
		},
		{
			name:                "twenty_failures",
			numContinuousFailed: 20,
			want:                38.337599924474700,
		},
		{ // 単純に計算すると300を超えるが、最大値は300
			name:                "caps_at_300_seconds",
			numContinuousFailed: 50,
			want:                300,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.InDeltaf(
				t,
				tt.want,
				CalculateRetryIntervalSec(RetryIntervalCalculationBase, tt.numContinuousFailed),
				0.1,
				"CalculateRetryIntervalSec(%v)",
				tt.numContinuousFailed,
			)
		})
	}
}
