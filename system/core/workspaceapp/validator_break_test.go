package workspaceapp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"app.modules/core/i18n"
	"app.modules/core/repository"
	"app.modules/core/utils"
)

func TestValidateBreakOptions(t *testing.T) {
	require.NoError(t, i18n.LoadLocaleFolderFS())

	app := WorkspaceApp{
		Configs: &Configs{
			Constants: repository.ConstantsConfigDoc{
				MinBreakDurationMin: 5,
				MaxBreakDurationMin: 60,
			},
		},
	}

	t.Run("accepts break without options", func(t *testing.T) {
		command, parseMessage := utils.ParseCommand("!break", false)
		require.Empty(t, parseMessage)
		require.Equal(t, utils.Break, command.CommandType)
		assert.Empty(t, app.ValidateBreak(*command))
	})

	t.Run("accepts duration option", func(t *testing.T) {
		command, parseMessage := utils.ParseCommand("!break min 20", false)
		require.Empty(t, parseMessage)
		require.Equal(t, utils.Break, command.CommandType)
		assert.Empty(t, app.ValidateBreak(*command))
	})

	t.Run("rejects work option during parsing", func(t *testing.T) {
		command, parseMessage := utils.ParseCommand("!break work coffee", false)
		require.NotEmpty(t, parseMessage)
		assert.Nil(t, command)
	})

	t.Run("rejects order option during parsing", func(t *testing.T) {
		command, parseMessage := utils.ParseCommand("!break order 1", false)
		require.NotEmpty(t, parseMessage)
		assert.Nil(t, command)
	})
}
