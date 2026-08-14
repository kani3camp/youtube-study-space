package workspaceapp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	i18nmsg "app.modules/core/i18n/typed"
	"app.modules/core/utils"
)

func TestParseAndValidateMessage_ValidCommand(t *testing.T) {
	loadWorkspaceAppTestI18n(t)
	app := WorkspaceApp{ProcessedUserDisplayName: "User One"}

	prepared := app.parseAndValidateMessage("!info", false)

	assert.False(t, prepared.SkipExecution)
	assert.Empty(t, prepared.ImmediateReply)
	require.NotNil(t, prepared.CommandDetails)
	assert.Equal(t, utils.Info, prepared.CommandDetails.CommandType)
}

func TestParseAndValidateMessage_ValidationReply(t *testing.T) {
	loadWorkspaceAppTestI18n(t)
	app := WorkspaceApp{ProcessedUserDisplayName: "User One"}

	prepared := app.parseAndValidateMessage("!more 0", false)

	assert.True(t, prepared.SkipExecution)
	assert.Nil(t, prepared.CommandDetails)
	assert.Equal(
		t,
		i18nmsg.CommonSir("User One")+i18nmsg.ValidateNonOneOrMoreExtendedTime(),
		prepared.ImmediateReply,
	)
}

func TestParseAndValidateMessage_NonCommandNeedsNoReply(t *testing.T) {
	loadWorkspaceAppTestI18n(t)
	app := WorkspaceApp{ProcessedUserDisplayName: "User One"}

	prepared := app.parseAndValidateMessage("hello study room", false)

	assert.False(t, prepared.SkipExecution)
	assert.Empty(t, prepared.ImmediateReply)
	require.NotNil(t, prepared.CommandDetails)
	assert.Equal(t, utils.NotCommand, prepared.CommandDetails.CommandType)
}
