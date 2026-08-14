package workspaceapp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"app.modules/core/repository"
)

func TestCanProcessDurableInboxMessageSupportsInvalidCommandWithoutMutatingActorState(t *testing.T) {
	app := WorkspaceApp{
		Configs: &Configs{
			Constants:            repository.ConstantsConfigDoc{YoutubeMembershipEnabled: true},
			LiveChatBotChannelID: "bot-channel",
		},
		ProcessedUserID:          "existing-actor",
		ProcessedUserDisplayName: "Existing Actor",
	}

	supported, err := app.CanProcessDurableInboxMessage(repository.LiveChatInboxDoc{
		AuthorChannelID:   "new-actor",
		AuthorDisplayName: "New Actor",
		MessageText:       "!definitely-unknown-command",
	})
	require.NoError(t, err)
	assert.True(t, supported)
	assert.Equal(t, "existing-actor", app.ProcessedUserID)
	assert.Equal(t, "Existing Actor", app.ProcessedUserDisplayName)
}

func TestCanProcessDurableInboxMessageRejectsExecutableCommandBeforeClaim(t *testing.T) {
	app := WorkspaceApp{
		Configs: &Configs{
			Constants: repository.ConstantsConfigDoc{YoutubeMembershipEnabled: true},
		},
	}

	supported, err := app.CanProcessDurableInboxMessage(repository.LiveChatInboxDoc{
		AuthorChannelID:   "actor",
		AuthorDisplayName: "Actor",
		MessageText:       "!out",
	})
	require.NoError(t, err)
	assert.False(t, supported)
}
