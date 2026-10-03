package mypage

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateChannelLinkOwnership_AllowsWhenNoExistingOwner(t *testing.T) {
	t.Parallel()

	err := validateChannelLinkOwnership("UCxxxxxxxxxxxxxxxxxxxxxx", "firebase-user-a", "")

	require.NoError(t, err)
}

func TestValidateChannelLinkOwnership_AllowsWhenSameOwner(t *testing.T) {
	t.Parallel()

	err := validateChannelLinkOwnership(
		"UCxxxxxxxxxxxxxxxxxxxxxx",
		"firebase-user-a",
		"firebase-user-a",
	)

	require.NoError(t, err)
}

func TestValidateChannelLinkOwnership_RejectsWhenDifferentOwner(t *testing.T) {
	t.Parallel()

	err := validateChannelLinkOwnership(
		"UCxxxxxxxxxxxxxxxxxxxxxx",
		"firebase-user-b",
		"firebase-user-a",
	)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrYouTubeChannelAlreadyLinked)
}

func TestValidateChannelLinkOwnership_RejectsEmptyChannelID(t *testing.T) {
	t.Parallel()

	err := validateChannelLinkOwnership("", "firebase-user-a", "")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidRequest)
}

func TestValidateChannelLinkOwnership_RejectsEmptyFirebaseUID(t *testing.T) {
	t.Parallel()

	err := validateChannelLinkOwnership("UCxxxxxxxxxxxxxxxxxxxxxx", "", "")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnauthorized)
}

func TestValidateChannelLinkOwnership_RejectsWhitespaceOnlyChannelID(t *testing.T) {
	t.Parallel()

	err := validateChannelLinkOwnership("   ", "firebase-user-a", "")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidRequest))
}
