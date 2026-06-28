package mypage

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeYouTubeChannelFetcher struct {
	viewer Viewer
	err    error
}

func (f *fakeYouTubeChannelFetcher) FetchMyChannel(
	_ context.Context,
	_ string,
) (Viewer, error) {
	return f.viewer, f.err
}

// fakeLinkedAccountStore mirrors production SaveLinkedYouTubeAccount behavior:
// it does not reject duplicate channel ownership on its own.
type fakeLinkedAccountStore struct {
	ownerByChannel map[string]string
	lookupErr      error
	saveErr        error
	saveCalled     bool
}

func (s *fakeLinkedAccountStore) SaveLinkedYouTubeAccount(
	_ context.Context,
	firebaseUID string,
	viewer Viewer,
) error {
	s.saveCalled = true
	if s.saveErr != nil {
		return s.saveErr
	}

	if s.ownerByChannel == nil {
		s.ownerByChannel = make(map[string]string)
	}
	s.ownerByChannel[viewer.YouTubeChannelID] = firebaseUID
	return nil
}

func (s *fakeLinkedAccountStore) FindFirebaseUIDByYouTubeChannelID(
	_ context.Context,
	youtubeChannelID string,
) (string, error) {
	if s.lookupErr != nil {
		return "", s.lookupErr
	}
	if s.ownerByChannel == nil {
		return "", nil
	}
	return s.ownerByChannel[youtubeChannelID], nil
}

func (s *fakeLinkedAccountStore) existingOwner(youtubeChannelID string) string {
	if s.ownerByChannel == nil {
		return ""
	}
	return s.ownerByChannel[youtubeChannelID]
}

func TestService_LinkYouTube_SucceedsOnFirstLink(t *testing.T) {
	t.Parallel()

	svc := NewService(nil, nil)
	viewer := Viewer{
		YouTubeChannelID: "UCxxxxxxxxxxxxxxxxxxxxxx",
		DisplayName:      "テストユーザー",
		ProfileImageURL:  "https://example.com/profile.png",
	}
	store := &fakeLinkedAccountStore{}

	resp, err := svc.LinkYouTube(
		context.Background(),
		AuthenticatedFirebaseUser{FirebaseUID: "firebase-user-a"},
		"youtube-access-token",
		&fakeYouTubeChannelFetcher{viewer: viewer},
		store,
	)

	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Status)
	assert.Equal(t, viewer, resp.Viewer)
	assert.Equal(t, "firebase-user-a", store.existingOwner(viewer.YouTubeChannelID))
	assert.True(t, store.saveCalled)
}

func TestService_LinkYouTube_AllowsRelinkBySameFirebaseUID(t *testing.T) {
	t.Parallel()

	channelID := "UCxxxxxxxxxxxxxxxxxxxxxx"
	svc := NewService(nil, nil)
	viewer := Viewer{
		YouTubeChannelID: channelID,
		DisplayName:      "テストユーザー",
		ProfileImageURL:  "https://example.com/profile.png",
	}
	store := &fakeLinkedAccountStore{
		ownerByChannel: map[string]string{
			channelID: "firebase-user-a",
		},
	}

	resp, err := svc.LinkYouTube(
		context.Background(),
		AuthenticatedFirebaseUser{FirebaseUID: "firebase-user-a"},
		"youtube-access-token",
		&fakeYouTubeChannelFetcher{viewer: viewer},
		store,
	)

	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Status)
	assert.Equal(t, "firebase-user-a", store.existingOwner(channelID))
	assert.True(t, store.saveCalled)
}

func TestService_LinkYouTube_RejectsWhenChannelLinkedToAnotherFirebaseUID(t *testing.T) {
	t.Parallel()

	channelID := "UCxxxxxxxxxxxxxxxxxxxxxx"
	svc := NewService(nil, nil)
	viewer := Viewer{
		YouTubeChannelID: channelID,
		DisplayName:      "別ユーザーのチャンネル",
		ProfileImageURL:  "https://example.com/other.png",
	}
	store := &fakeLinkedAccountStore{
		ownerByChannel: map[string]string{
			channelID: "firebase-user-a",
		},
	}

	_, err := svc.LinkYouTube(
		context.Background(),
		AuthenticatedFirebaseUser{FirebaseUID: "firebase-user-b"},
		"youtube-access-token",
		&fakeYouTubeChannelFetcher{viewer: viewer},
		store,
	)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrYouTubeChannelAlreadyLinked)
	assert.Equal(t, "firebase-user-a", store.existingOwner(channelID))
	assert.False(t, store.saveCalled)
}

func TestService_LinkYouTube_ReturnsErrorWhenOwnerLookupFails(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("firestore unavailable")
	svc := NewService(nil, nil)
	viewer := Viewer{
		YouTubeChannelID: "UCxxxxxxxxxxxxxxxxxxxxxx",
		DisplayName:      "テストユーザー",
		ProfileImageURL:  "https://example.com/profile.png",
	}
	store := &fakeLinkedAccountStore{
		lookupErr: expectedErr,
	}

	_, err := svc.LinkYouTube(
		context.Background(),
		AuthenticatedFirebaseUser{FirebaseUID: "firebase-user-a"},
		"youtube-access-token",
		&fakeYouTubeChannelFetcher{viewer: viewer},
		store,
	)

	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
	assert.False(t, store.saveCalled)
}
