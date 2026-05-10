package workspaceapp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func newWorkspaceApp() *WorkspaceApp {
	return &WorkspaceApp{}
}

func assertProcessedUserInitialState(t *testing.T, app *WorkspaceApp) {
	t.Helper()

	assert.Equal(t, "", app.ProcessedUserID)
	assert.Equal(t, "", app.ProcessedUserDisplayName)
	assert.Equal(t, "", app.ProcessedUserProfileImageURL)
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

			assert.Equal(t, userID, app.ProcessedUserID)
			assert.Equal(t, userDisplayName, app.ProcessedUserDisplayName)
			assert.Equal(t, userProfileImageURL, app.ProcessedUserProfileImageURL)
			assert.Equal(t, tt.wantModeratorRole, app.ProcessedUserIsModeratorOrOwner)
			assert.Equal(t, tt.isChatMember, app.ProcessedUserIsMember)
		})
	}
}
