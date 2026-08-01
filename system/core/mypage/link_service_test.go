package mypage

import (
	"context"
	"errors"
	"sync"
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

type fakeLinkedAccountStore struct {
	mu             sync.Mutex
	ownerByChannel map[string]string
	linkErr        error
	linkCalled     bool
}

func (s *fakeLinkedAccountStore) LinkYouTubeAccount(
	_ context.Context,
	firebaseUID string,
	viewer Viewer,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.linkCalled = true
	if s.linkErr != nil {
		return s.linkErr
	}

	if s.ownerByChannel == nil {
		s.ownerByChannel = make(map[string]string)
	}
	if err := validateChannelLinkOwnership(
		viewer.YouTubeChannelID,
		firebaseUID,
		s.ownerByChannel[viewer.YouTubeChannelID],
	); err != nil {
		return err
	}
	s.ownerByChannel[viewer.YouTubeChannelID] = firebaseUID
	return nil
}

func (s *fakeLinkedAccountStore) existingOwner(youtubeChannelID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ownerByChannel == nil {
		return ""
	}
	return s.ownerByChannel[youtubeChannelID]
}

func TestService_LinkYouTube_SucceedsOnFirstLink(t *testing.T) {
	t.Parallel()

	svc := newTestService(t, &fakeStore{}, nil)
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
	assert.True(t, store.linkCalled)
}

func TestService_LinkYouTube_AllowsRelinkBySameFirebaseUID(t *testing.T) {
	t.Parallel()

	channelID := "UCxxxxxxxxxxxxxxxxxxxxxx"
	svc := newTestService(t, &fakeStore{}, nil)
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
	assert.True(t, store.linkCalled)
}

func TestService_LinkYouTube_RejectsWhenChannelLinkedToAnotherFirebaseUID(t *testing.T) {
	t.Parallel()

	channelID := "UCxxxxxxxxxxxxxxxxxxxxxx"
	svc := newTestService(t, &fakeStore{}, nil)
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
	assert.True(t, store.linkCalled)
}

func TestService_LinkYouTube_ConcurrentRequestsAllowOnlyOneOwner(t *testing.T) {
	t.Parallel()

	svc := newTestService(t, &fakeStore{}, nil)
	viewer := Viewer{YouTubeChannelID: "UCxxxxxxxxxxxxxxxxxxxxxx"}
	store := &fakeLinkedAccountStore{}
	firebaseUIDs := []string{"firebase-user-a", "firebase-user-b"}
	errs := make([]error, len(firebaseUIDs))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, firebaseUID := range firebaseUIDs {
		wg.Add(1)
		go func(index int, uid string) {
			defer wg.Done()
			<-start
			_, errs[index] = svc.LinkYouTube(
				context.Background(),
				AuthenticatedFirebaseUser{FirebaseUID: uid},
				"youtube-access-token",
				&fakeYouTubeChannelFetcher{viewer: viewer},
				store,
			)
		}(i, firebaseUID)
	}
	close(start)
	wg.Wait()

	var successCount int
	var conflictCount int
	for _, err := range errs {
		switch {
		case err == nil:
			successCount++
		case errors.Is(err, ErrYouTubeChannelAlreadyLinked):
			conflictCount++
		default:
			t.Fatalf("unexpected link error: %v", err)
		}
	}
	assert.Equal(t, 1, successCount)
	assert.Equal(t, 1, conflictCount)
	assert.Contains(t, firebaseUIDs, store.existingOwner(viewer.YouTubeChannelID))
}

func TestService_LinkYouTube_ReturnsErrorWhenAtomicLinkFails(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("firestore unavailable")
	svc := newTestService(t, &fakeStore{}, nil)
	viewer := Viewer{
		YouTubeChannelID: "UCxxxxxxxxxxxxxxxxxxxxxx",
		DisplayName:      "テストユーザー",
		ProfileImageURL:  "https://example.com/profile.png",
	}
	store := &fakeLinkedAccountStore{
		linkErr: expectedErr,
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
	assert.True(t, store.linkCalled)
}
