package mypage

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMeGetter struct {
	response Response
	err      error

	linkResponse LinkYouTubeResponse
	linkErr      error

	called   bool
	identity Identity

	linkCalled             bool
	linkAuthenticatedUser  AuthenticatedFirebaseUser
	linkYouTubeAccessToken string
}

func (g *fakeMeGetter) GetMe(_ context.Context, identity Identity) (Response, error) {
	g.called = true
	g.identity = identity
	return g.response, g.err
}

func (g *fakeMeGetter) LinkYouTube(
	_ context.Context,
	authenticatedUser AuthenticatedFirebaseUser,
	youtubeAccessToken string,
	_ YouTubeChannelFetcher,
	_ LinkedAccountStore,
) (LinkYouTubeResponse, error) {
	g.linkCalled = true
	g.linkAuthenticatedUser = authenticatedUser
	g.linkYouTubeAccessToken = youtubeAccessToken
	if g.linkErr != nil {
		return LinkYouTubeResponse{}, g.linkErr
	}
	if strings.TrimSpace(youtubeAccessToken) == "" {
		return LinkYouTubeResponse{}, ErrInvalidRequest
	}
	return g.linkResponse, nil
}

type fakeIdentityResolver struct {
	identity Identity
	err      error

	called bool
}

func (r *fakeIdentityResolver) Resolve(_ context.Context, _ *http.Request) (Identity, error) {
	r.called = true
	return r.identity, r.err
}

type fakeFirebaseAuthenticator struct {
	user AuthenticatedFirebaseUser
	err  error
}

func (a *fakeFirebaseAuthenticator) Authenticate(
	_ context.Context,
	_ FirebaseIDTokenRequest,
) (AuthenticatedFirebaseUser, error) {
	return a.user, a.err
}

func TestHandler_GetMe_ReturnsOK(t *testing.T) {
	t.Parallel()

	getter := &fakeMeGetter{
		response: Response{
			Status: StatusOK,
			Viewer: Viewer{
				YouTubeChannelID: "UCxxxxxxxxxxxxxxxxxxxxxx",
				DisplayName:      "テストユーザー",
				ProfileImageURL:  "https://example.com/profile.png",
			},
			Stats: &Stats{
				DailyWorkSec:      120,
				CumulativeWorkSec: 3600,
			},
			CurrentSeat: nil,
		},
	}
	resolver := &fakeIdentityResolver{
		identity: testIdentity(),
	}

	handler := NewHandler(HandlerOptions{
		Service:          getter,
		IdentityResolver: resolver,
		AllowedOrigin:    "https://mypage.example.com",
	})

	req := httptest.NewRequest(http.MethodGet, "/mypage/me", nil)
	req.Header.Set("Origin", "https://mypage.example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "https://mypage.example.com", rec.Header().Get("Access-Control-Allow-Origin"))

	var got Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))

	assert.Equal(t, StatusOK, got.Status)
	assert.Equal(t, "UCxxxxxxxxxxxxxxxxxxxxxx", got.Viewer.YouTubeChannelID)
	require.NotNil(t, got.Stats)
	assert.Equal(t, 120, got.Stats.DailyWorkSec)
	assert.Equal(t, 3600, got.Stats.CumulativeWorkSec)
	assert.Nil(t, got.CurrentSeat)

	assert.True(t, resolver.called)
	assert.True(t, getter.called)
	assert.Equal(t, "UCxxxxxxxxxxxxxxxxxxxxxx", getter.identity.YouTubeChannelID)
}

func TestHandler_GetMe_ReturnsUnauthorizedWhenIdentityResolverFails(t *testing.T) {
	t.Parallel()

	getter := &fakeMeGetter{}
	resolver := &fakeIdentityResolver{
		err: ErrUnauthorized,
	}

	handler := NewHandler(HandlerOptions{
		Service:          getter,
		IdentityResolver: resolver,
		AllowedOrigin:    "",
	})

	req := httptest.NewRequest(http.MethodGet, "/mypage/me", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, getter.called)

	var got ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "unauthorized", got.Error.Code)
}

func TestHandler_GetMe_ReturnsInternalErrorWhenServiceFails(t *testing.T) {
	t.Parallel()

	getter := &fakeMeGetter{
		err: errors.New("service failed"),
	}
	resolver := &fakeIdentityResolver{
		identity: testIdentity(),
	}

	handler := NewHandler(HandlerOptions{
		Service:          getter,
		IdentityResolver: resolver,
		AllowedOrigin:    "",
	})

	req := httptest.NewRequest(http.MethodGet, "/mypage/me", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)

	var got ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "internal_error", got.Error.Code)
}

func TestHandler_ReturnsNotFoundForUnknownPath(t *testing.T) {
	t.Parallel()

	getter := &fakeMeGetter{}
	resolver := &fakeIdentityResolver{
		identity: testIdentity(),
	}

	handler := NewHandler(HandlerOptions{
		Service:          getter,
		IdentityResolver: resolver,
		AllowedOrigin:    "",
	})

	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.False(t, resolver.called)
	assert.False(t, getter.called)

	var got ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "not_found", got.Error.Code)
}

func TestHandler_ReturnsMethodNotAllowed(t *testing.T) {
	t.Parallel()

	getter := &fakeMeGetter{}
	resolver := &fakeIdentityResolver{
		identity: testIdentity(),
	}

	handler := NewHandler(HandlerOptions{
		Service:          getter,
		IdentityResolver: resolver,
		AllowedOrigin:    "",
	})

	req := httptest.NewRequest(http.MethodPost, "/mypage/me", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	assert.False(t, resolver.called)
	assert.False(t, getter.called)

	var got ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "method_not_allowed", got.Error.Code)
}

func TestHandler_ReturnsNoContentForOptions(t *testing.T) {
	t.Parallel()

	getter := &fakeMeGetter{}
	resolver := &fakeIdentityResolver{}

	handler := NewHandler(HandlerOptions{
		Service:          getter,
		IdentityResolver: resolver,
		AllowedOrigin:    "https://mypage.example.com",
	})

	req := httptest.NewRequest(http.MethodOptions, "/mypage/me", nil)
	req.Header.Set("Origin", "https://mypage.example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "https://mypage.example.com", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, OPTIONS", rec.Header().Get("Access-Control-Allow-Methods"))
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), "Authorization")
	assert.False(t, resolver.called)
	assert.False(t, getter.called)
}

func TestHandler_PostYouTubeLink_ReturnsUnauthorizedWithoutBearer(t *testing.T) {
	t.Parallel()

	getter := &fakeMeGetter{}
	handler := NewHandler(HandlerOptions{
		Service:               getter,
		FirebaseAuthenticator: &fakeFirebaseAuthenticator{err: ErrUnauthorized},
	})

	req := httptest.NewRequest(http.MethodPost, "/mypage/auth/youtube-link", strings.NewReader(`{"youtubeAccessToken":"token"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, getter.linkCalled)

	var got ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "unauthorized", got.Error.Code)
}

func TestHandler_PostYouTubeLink_ReturnsBadRequestWhenBodyMissingToken(t *testing.T) {
	t.Parallel()

	getter := &fakeMeGetter{}
	handler := NewHandler(HandlerOptions{
		Service: getter,
		FirebaseAuthenticator: &fakeFirebaseAuthenticator{
			user: AuthenticatedFirebaseUser{FirebaseUID: "firebase-user-a"},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/mypage/auth/youtube-link", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer firebase-id-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.True(t, getter.linkCalled)

	var got ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "invalid_request", got.Error.Code)
}

func TestHandler_PostYouTubeLink_ReturnsConflictWhenChannelAlreadyLinked(t *testing.T) {
	t.Parallel()

	getter := &fakeMeGetter{
		linkErr: ErrYouTubeChannelAlreadyLinked,
	}
	handler := NewHandler(HandlerOptions{
		Service: getter,
		FirebaseAuthenticator: &fakeFirebaseAuthenticator{
			user: AuthenticatedFirebaseUser{FirebaseUID: "firebase-user-b"},
		},
	})

	req := httptest.NewRequest(
		http.MethodPost,
		"/mypage/auth/youtube-link",
		strings.NewReader(`{"youtubeAccessToken":"youtube-access-token"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer firebase-id-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
	assert.True(t, getter.linkCalled)

	var got ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "channel_already_linked", got.Error.Code)
}

func TestHandler_PostYouTubeLink_ReturnsBadRequestWhenBodyTooLarge(t *testing.T) {
	t.Parallel()

	getter := &fakeMeGetter{}
	handler := NewHandler(HandlerOptions{
		Service: getter,
		FirebaseAuthenticator: &fakeFirebaseAuthenticator{
			user: AuthenticatedFirebaseUser{FirebaseUID: "firebase-user-a"},
		},
	})

	largeToken := strings.Repeat("a", maxJSONBodyBytes)
	req := httptest.NewRequest(
		http.MethodPost,
		"/mypage/auth/youtube-link",
		strings.NewReader(`{"youtubeAccessToken":"`+largeToken+`"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer firebase-id-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.False(t, getter.linkCalled)

	var got ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "invalid_request", got.Error.Code)
}

func TestHandler_GetMe_DoesNotSetAccessControlAllowOriginForUntrustedOrigin(t *testing.T) {
	t.Parallel()

	identity := testIdentity()
	getter := &fakeMeGetter{
		response: Response{
			Status: StatusOK,
			Viewer: viewerFromIdentity(identity),
		},
	}
	resolver := &fakeIdentityResolver{identity: identity}

	handler := NewHandler(HandlerOptions{
		Service:          getter,
		IdentityResolver: resolver,
		AllowedOrigin:    "https://mypage.example.com",
	})

	req := httptest.NewRequest(http.MethodGet, "/mypage/me", nil)
	req.Header.Set("Origin", "https://evil.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestHandler_Options_DoesNotSetCORSForUntrustedOrigin(t *testing.T) {
	t.Parallel()

	handler := NewHandler(HandlerOptions{
		AllowedOrigin: "https://mypage.example.com",
	})

	req := httptest.NewRequest(http.MethodOptions, "/mypage/me", nil)
	req.Header.Set("Origin", "https://evil.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func viewerFromIdentity(identity Identity) Viewer {
	return Viewer{
		YouTubeChannelID: identity.YouTubeChannelID,
		DisplayName:      identity.DisplayName,
		ProfileImageURL:  identity.ProfileImageURL,
	}
}
