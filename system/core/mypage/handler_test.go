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

	handler := newTestHandler(t, HandlerOptions{
		MeGetter:         getter,
		YouTubeLinker:    getter,
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

	handler := newTestHandler(t, HandlerOptions{
		MeGetter:         getter,
		YouTubeLinker:    getter,
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

	handler := newTestHandler(t, HandlerOptions{
		MeGetter:         getter,
		YouTubeLinker:    getter,
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

func TestHandler_GetMe_MapsServiceError(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(t, HandlerOptions{
		MeGetter: &fakeMeGetter{err: ErrYouTubeLinkRequired},
		IdentityResolver: &fakeIdentityResolver{
			identity: testIdentity(),
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/mypage/me", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
	var got ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "link_required", got.Error.Code)
}

func TestHandler_ReturnsNotFoundForUnknownPath(t *testing.T) {
	t.Parallel()

	getter := &fakeMeGetter{}
	resolver := &fakeIdentityResolver{
		identity: testIdentity(),
	}

	handler := newTestHandler(t, HandlerOptions{
		MeGetter:         getter,
		YouTubeLinker:    getter,
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

	handler := newTestHandler(t, HandlerOptions{
		MeGetter:         getter,
		YouTubeLinker:    getter,
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

	handler := newTestHandler(t, HandlerOptions{
		MeGetter:         getter,
		YouTubeLinker:    getter,
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
	handler := newTestHandler(t, HandlerOptions{
		MeGetter:              getter,
		YouTubeLinker:         getter,
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
	handler := newTestHandler(t, HandlerOptions{
		MeGetter:      getter,
		YouTubeLinker: getter,
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
	handler := newTestHandler(t, HandlerOptions{
		MeGetter:      getter,
		YouTubeLinker: getter,
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

func TestHandler_PostYouTubeLink_MapsYouTubeErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		statusCode int
		code       string
	}{
		{name: "invalid token", err: ErrInvalidYouTubeAccessToken, statusCode: http.StatusBadRequest, code: "invalid_youtube_access_token"},
		{name: "channel not found", err: ErrYouTubeChannelNotFound, statusCode: http.StatusBadGateway, code: "youtube_channel_not_found"},
		{name: "forbidden", err: ErrYouTubeForbidden, statusCode: http.StatusForbidden, code: "forbidden"},
		{name: "rate limited", err: ErrYouTubeRateLimited, statusCode: http.StatusTooManyRequests, code: "rate_limited"},
		{name: "upstream", err: ErrYouTubeUpstream, statusCode: http.StatusBadGateway, code: "upstream_auth_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler := newTestHandler(t, HandlerOptions{
				YouTubeLinker: &fakeMeGetter{linkErr: tt.err},
				FirebaseAuthenticator: &fakeFirebaseAuthenticator{
					user: AuthenticatedFirebaseUser{FirebaseUID: "firebase-user-a"},
				},
			})

			req := httptest.NewRequest(
				http.MethodPost,
				"/mypage/auth/youtube-link",
				strings.NewReader(`{"youtubeAccessToken":"youtube-access-token"}`),
			)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			require.Equal(t, tt.statusCode, rec.Code)
			var got ErrorResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
			assert.Equal(t, tt.code, got.Error.Code)
		})
	}
}

func TestHandler_PostYouTubeLink_ReturnsBadRequestWhenBodyTooLarge(t *testing.T) {
	t.Parallel()

	getter := &fakeMeGetter{}
	handler := newTestHandler(t, HandlerOptions{
		MeGetter:      getter,
		YouTubeLinker: getter,
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

	handler := newTestHandler(t, HandlerOptions{
		MeGetter:         getter,
		YouTubeLinker:    getter,
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

	handler := newTestHandler(t, HandlerOptions{
		AllowedOrigin: "https://mypage.example.com",
	})

	req := httptest.NewRequest(http.MethodOptions, "/mypage/me", nil)
	req.Header.Set("Origin", "https://evil.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestNewHandler_RejectsNilDependencies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		remove func(*HandlerOptions)
	}{
		{name: "me getter", remove: func(options *HandlerOptions) { options.MeGetter = nil }},
		{name: "youtube linker", remove: func(options *HandlerOptions) { options.YouTubeLinker = nil }},
		{name: "identity resolver", remove: func(options *HandlerOptions) { options.IdentityResolver = nil }},
		{name: "firebase authenticator", remove: func(options *HandlerOptions) { options.FirebaseAuthenticator = nil }},
		{name: "channel fetcher", remove: func(options *HandlerOptions) { options.ChannelFetcher = nil }},
		{name: "linked account store", remove: func(options *HandlerOptions) { options.LinkedAccountStore = nil }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			options := validHandlerOptions()
			tt.remove(&options)

			handler, err := NewHandler(options)

			require.Error(t, err)
			assert.Nil(t, handler)
			assert.Contains(t, err.Error(), tt.name)
		})
	}
}

func newTestHandler(t *testing.T, options HandlerOptions) http.Handler {
	t.Helper()

	defaults := validHandlerOptions()
	if options.MeGetter == nil {
		options.MeGetter = defaults.MeGetter
	}
	if options.YouTubeLinker == nil {
		options.YouTubeLinker = defaults.YouTubeLinker
	}
	if options.IdentityResolver == nil {
		options.IdentityResolver = defaults.IdentityResolver
	}
	if options.FirebaseAuthenticator == nil {
		options.FirebaseAuthenticator = defaults.FirebaseAuthenticator
	}
	if options.ChannelFetcher == nil {
		options.ChannelFetcher = defaults.ChannelFetcher
	}
	if options.LinkedAccountStore == nil {
		options.LinkedAccountStore = defaults.LinkedAccountStore
	}

	handler, err := NewHandler(options)
	require.NoError(t, err)
	return handler
}

func validHandlerOptions() HandlerOptions {
	service := &fakeMeGetter{}
	return HandlerOptions{
		MeGetter:              service,
		YouTubeLinker:         service,
		IdentityResolver:      &fakeIdentityResolver{identity: testIdentity()},
		FirebaseAuthenticator: &fakeFirebaseAuthenticator{user: AuthenticatedFirebaseUser{FirebaseUID: "firebase-user-a"}},
		ChannelFetcher:        &fakeYouTubeChannelFetcher{},
		LinkedAccountStore:    &fakeLinkedAccountStore{},
	}
}

func viewerFromIdentity(identity Identity) Viewer {
	return Viewer{
		YouTubeChannelID: identity.YouTubeChannelID,
		DisplayName:      identity.DisplayName,
		ProfileImageURL:  identity.ProfileImageURL,
	}
}
