package mypage

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
)

const (
	pathMe           = "/mypage/me"
	pathYouTubeLink  = "/mypage/auth/youtube-link"
	maxJSONBodyBytes = 64 * 1024
)

type MeGetter interface {
	GetMe(ctx context.Context, identity Identity) (Response, error)
	LinkYouTube(
		ctx context.Context,
		authenticatedUser AuthenticatedFirebaseUser,
		youtubeAccessToken string,
		channelFetcher YouTubeChannelFetcher,
		linkedAccountStore LinkedAccountStore,
	) (LinkYouTubeResponse, error)
}

type Handler struct {
	service               MeGetter
	identityResolver      IdentityResolver
	firebaseAuthenticator FirebaseAuthenticator
	channelFetcher        YouTubeChannelFetcher
	linkedAccountStore    LinkedAccountStore
	allowedOrigin         string
}

type HandlerOptions struct {
	Service               MeGetter
	IdentityResolver      IdentityResolver
	FirebaseAuthenticator FirebaseAuthenticator
	ChannelFetcher        YouTubeChannelFetcher
	LinkedAccountStore    LinkedAccountStore
	AllowedOrigin         string
}

func NewHandler(options HandlerOptions) http.Handler {
	return &Handler{
		service:               options.Service,
		identityResolver:      options.IdentityResolver,
		firebaseAuthenticator: options.FirebaseAuthenticator,
		channelFetcher:        options.ChannelFetcher,
		linkedAccountStore:    options.LinkedAccountStore,
		allowedOrigin:         options.AllowedOrigin,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.setHeaders(w, r)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch r.URL.Path {
	case pathMe:
		h.handleGetMe(w, r)
	case pathYouTubeLink:
		h.handlePostYouTubeLink(w, r)
	default:
		writeError(w, http.StatusNotFound, "not_found", "not found")
	}
}

func (h *Handler) handleGetMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	identity, err := h.identityResolver.Resolve(r.Context(), r)
	if err != nil {
		h.writeMappedError(w, r, err)
		return
	}

	resp, err := h.service.GetMe(r.Context(), identity)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get mypage", appendClientLogAttrs(r)...)
		slog.ErrorContext(r.Context(), "mypage service error", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handlePostYouTubeLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	authenticatedUser, err := h.firebaseAuthenticator.Authenticate(r.Context(), requestAuthHeader{request: r})
	if err != nil {
		h.writeMappedError(w, r, err)
		return
	}

	body, err := readJSONBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid request")
		return
	}

	var req LinkYouTubeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid request")
		return
	}

	resp, err := h.service.LinkYouTube(
		r.Context(),
		authenticatedUser,
		req.YouTubeAccessToken,
		h.channelFetcher,
		h.linkedAccountStore,
	)
	if err != nil {
		h.writeMappedError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) writeMappedError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")

	case errors.Is(err, ErrYouTubeLinkRequired):
		writeError(w, http.StatusConflict, "link_required", "youtube account link required")

	case errors.Is(err, ErrYouTubeChannelAlreadyLinked):
		writeError(w, http.StatusConflict, "channel_already_linked", "youtube channel already linked")

	case errors.Is(err, ErrInvalidRequest):
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid request")

	case errors.Is(err, ErrInvalidYouTubeAccessToken):
		writeError(w, http.StatusBadRequest, "invalid_youtube_access_token", "invalid youtube access token")

	case errors.Is(err, ErrInvalidIdentity):
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")

	default:
		slog.ErrorContext(r.Context(), "failed to handle mypage request", append(appendClientLogAttrs(r), "err", err)...)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func (h *Handler) setHeaders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	if h.allowedOrigin == "" {
		return
	}

	origin := r.Header.Get("Origin")
	switch {
	case h.allowedOrigin == "*":
		w.Header().Set("Access-Control-Allow-Origin", "*")
	case origin != "" && origin == h.allowedOrigin:
		w.Header().Set("Access-Control-Allow-Origin", origin)
	default:
		return
	}

	w.Header().Set("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set(
		"Access-Control-Allow-Headers",
		"Authorization, Content-Type, X-Client-App, X-Client-Version, X-Client-Request-Id, X-Client-Build-Time, X-Client-Timezone, X-Client-Platform",
	)
}

func readJSONBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()

	return io.ReadAll(http.MaxBytesReader(nil, r.Body, maxJSONBodyBytes))
}

func writeJSON(w http.ResponseWriter, statusCode int, value any) {
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("failed to encode JSON response", "err", err)
	}
}

func writeError(w http.ResponseWriter, statusCode int, code string, message string) {
	writeJSON(w, statusCode, ErrorResponse{
		Error: APIError{
			Code:    code,
			Message: message,
		},
	})
}

type requestAuthHeader struct {
	request *http.Request
}

func (r requestAuthHeader) AuthorizationHeader() string {
	if r.request == nil {
		return ""
	}
	return r.request.Header.Get("Authorization")
}

func appendClientLogAttrs(r *http.Request) []any {
	if r == nil {
		return nil
	}

	return []any{
		"path", r.URL.Path,
		"method", r.Method,
		"client_app", headerOrUnknown(r, "X-Client-App"),
		"client_version", headerOrUnknown(r, "X-Client-Version"),
		"client_request_id", headerOrUnknown(r, "X-Client-Request-Id"),
		"client_build_time", headerOrUnknown(r, "X-Client-Build-Time"),
		"client_timezone", headerOrUnknown(r, "X-Client-Timezone"),
		"accept_language", headerOrUnknown(r, "Accept-Language"),
		"user_agent", headerOrUnknown(r, "User-Agent"),
	}
}

func headerOrUnknown(r *http.Request, name string) string {
	value := r.Header.Get(name)
	if value == "" {
		return "unknown"
	}
	return value
}
