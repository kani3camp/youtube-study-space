package mypage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type (
	VerifiedIdentity struct{ UID, Provider string }
	RequestVerifier  interface {
		VerifyAppCheck(context.Context, string) error
		VerifyIDToken(context.Context, string) (VerifiedIdentity, error)
	}
)

type HTTPHandler struct {
	Auth     *AuthService
	BFF      *BFF
	Verifier RequestVerifier
	// One fixed HTTPS origin per environment. Never derive it from request
	// headers; actual Hosting/proxy host forwarding must be verified at deploy.
	PublicOrigin string
	// A deployment may inject an extractor only after verifying its actual
	// trusted proxy chain. Default ignores all client forwarding headers.
	RequestIP func(*http.Request) (string, error)
	limiter   processLimiter
}

var routeMethods = map[string]string{
	"/api/auth/youtube/start": "POST", "/api/auth/youtube/callback": "GET",
	"/api/auth/youtube/channel": "GET", "/api/auth/youtube/confirm": "POST",
	"/api/auth/session/complete": "POST", "/api/mypage": "GET",
}

func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID, err := opaque()
	if err != nil {
		requestID = "unavailable"
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Request-Id", requestID)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	method, known := routeMethods[r.URL.Path]
	if !known {
		writeAPIError(w, http.StatusNotFound, "INVALID_REQUEST", requestID)
		return
	}
	if r.Method != method {
		w.Header().Set("Allow", method)
		writeAPIError(w, http.StatusMethodNotAllowed, "INVALID_REQUEST", requestID)
		return
	}
	callback := r.URL.Path == "/api/auth/youtube/callback"
	if !callback && r.URL.RawQuery != "" {
		writeAPIError(w, 400, "INVALID_REQUEST", requestID)
		return
	}
	if r.Method == http.MethodGet && (r.ContentLength > 0 || len(r.TransferEncoding) > 0) {
		writeAPIError(w, 400, "INVALID_REQUEST", requestID)
		return
	}
	if h.Auth == nil || h.Verifier == nil || h.Auth.Now == nil {
		writeAPIError(w, 503, "TEMPORARY_UNAVAILABLE", requestID)
		return
	}
	if err := h.checkRequestOrigin(r); err != nil {
		code := errorCode(err)
		writeAPIError(w, statusFor(code), code, requestID)
		return
	}
	if sessionCookieCount(r) > 1 {
		writeAPIError(w, 400, "INVALID_REQUEST", requestID)
		return
	}
	budget := 5 * time.Second
	switch r.URL.Path {
	case "/api/auth/youtube/callback":
		budget = 15 * time.Second
	case "/api/auth/youtube/confirm":
		budget = 8 * time.Second
	case "/api/mypage":
		budget = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), budget)
	defer cancel()
	r = r.WithContext(ctx)
	var start StartRequest
	var confirmation struct {
		ConfirmationRef string `json:"confirmationRef"`
	}
	if r.Method == http.MethodPost {
		var target any
		switch r.URL.Path {
		case "/api/auth/youtube/start":
			target = &start
		case "/api/auth/youtube/confirm":
			target = &confirmation
		}
		if err := strictBody(w, r, target); err != nil {
			writeAPIError(w, statusFor(errorCode(err)), errorCode(err), requestID)
			return
		}
	}
	now := h.Auth.Now().UTC()
	ip, err := h.clientIP(r)
	if err != nil {
		writeAPIError(w, 400, "INVALID_REQUEST", requestID)
		return
	}
	// Bound rejected-token traffic before either verification dependency. A
	// process-local uid bucket alone cannot protect unauthenticated failures.
	if ok, retry := h.limiter.allow("coarse-ip:"+ip, 60, 20, now); !ok {
		rateError(w, retry, requestID)
		return
	}
	if r.URL.Path == "/api/auth/youtube/start" {
		if ok, retry := h.limiter.allowStart(ip, now); !ok {
			rateError(w, retry, requestID)
			return
		}
	}
	if callback {
		h.callback(w, r, requestID)
		return
	}
	appCheck := r.Header.Get("X-Firebase-AppCheck")
	if appCheck == "" || h.Verifier.VerifyAppCheck(ctx, appCheck) != nil {
		writeAPIError(w, 403, "APP_CHECK_REQUIRED", requestID)
		return
	}
	authenticated := r.URL.Path == "/api/mypage" || r.URL.Path == "/api/auth/session/complete"
	var identity VerifiedIdentity
	var account WebAccount
	if authenticated {
		bearer := r.Header.Get("Authorization")
		if !strings.HasPrefix(bearer, "Bearer ") || strings.Contains(strings.TrimPrefix(bearer, "Bearer "), " ") {
			writeAPIError(w, 401, "AUTH_REQUIRED", requestID)
			return
		}
		identity, err = h.Verifier.VerifyIDToken(ctx, strings.TrimPrefix(bearer, "Bearer "))
		if err != nil || identity.UID == "" || strings.Contains(identity.UID, "/") || identity.Provider != "custom" {
			writeAPIError(w, 401, "AUTH_REQUIRED", requestID)
			return
		}
		if ok, retry := h.limiter.allow("global:"+identity.UID, 30, 15, now); !ok {
			rateError(w, retry, requestID)
			return
		}
		if r.URL.Path == "/api/mypage" {
			if ok, retry := h.limiter.allow("mypage:"+identity.UID, 10, 10, now); !ok {
				rateError(w, retry, requestID)
				return
			}
		}
		account, err = h.Auth.Store.ReadAccount(ctx, identity.UID)
		if err != nil {
			if h.BFF != nil {
				h.BFF.Invalidate(identity.UID)
			}
			code := errorCode(err)
			if code == "INTERNAL_ERROR" {
				code = "TEMPORARY_UNAVAILABLE"
			}
			writeAPIError(w, statusFor(code), code, requestID)
			return
		}
		if err := checkPolicy(account, h.Auth.Policy); err != nil {
			if h.BFF != nil {
				h.BFF.Invalidate(identity.UID)
			}
			writeAPIError(w, statusFor(errorCode(err)), errorCode(err), requestID)
			return
		}
	}
	switch r.URL.Path {
	case "/api/auth/youtube/start":
		response, id, err := h.Auth.Start(ctx, transactionID(r), start)
		if err != nil {
			writeAPIError(w, statusFor(errorCode(err)), errorCode(err), requestID)
			return
		}
		setTransactionCookie(w, id, 600)
		writeJSON(w, 200, response)
	case "/api/auth/youtube/channel":
		response, err := h.Auth.Channel(ctx, transactionID(r))
		if err != nil {
			writeAPIError(w, statusFor(errorCode(err)), errorCode(err), requestID)
			return
		}
		writeJSON(w, 200, response)
	case "/api/auth/youtube/confirm":
		token, err := h.Auth.Confirm(ctx, transactionID(r), confirmation.ConfirmationRef)
		if err != nil {
			code := errorCode(err)
			if code == "TEMPORARY_UNAVAILABLE" || code == "OAUTH_TRANSACTION_CONSUMED" {
				setTransactionCookie(w, "", -1)
			}
			writeAPIError(w, statusFor(code), code, requestID)
			return
		}
		setTransactionCookie(w, "", -1)
		writeJSON(w, 200, struct {
			CustomToken string `json:"customToken"`
		}{token})
	case "/api/auth/session/complete":
		if err := h.Auth.Store.CompleteSession(ctx, identity.UID, h.Auth.Policy, now); err != nil {
			code := errorCode(err)
			if code == "INTERNAL_ERROR" {
				code = "TEMPORARY_UNAVAILABLE"
			}
			writeAPIError(w, statusFor(code), code, requestID)
			return
		}
		w.WriteHeader(204)
	case "/api/mypage":
		if h.BFF == nil {
			writeAPIError(w, 503, "TEMPORARY_UNAVAILABLE", requestID)
			return
		}
		response, err := h.BFF.Get(ctx, identity.UID, account)
		if err != nil {
			writeAPIError(w, statusFor(errorCode(err)), errorCode(err), requestID)
			return
		}
		writeJSON(w, 200, response)
	}
}

func (h *HTTPHandler) checkRequestOrigin(r *http.Request) error {
	configured, err := url.Parse(h.PublicOrigin)
	if err != nil || configured.Scheme != "https" || configured.Host == "" || configured.User != nil || configured.Path != "" || configured.RawQuery != "" || configured.Fragment != "" {
		return apiError("TEMPORARY_UNAVAILABLE")
	}
	if !strings.EqualFold(r.Host, configured.Host) || (r.URL.Host != "" && !strings.EqualFold(r.URL.Host, configured.Host)) {
		return apiError("INVALID_REQUEST")
	}
	origins := r.Header.Values("Origin")
	if len(origins) == 0 && r.Method == http.MethodGet {
		return nil
	}
	if len(origins) != 1 || origins[0] != h.PublicOrigin {
		return apiError("INVALID_REQUEST")
	}
	return nil
}

func (h *HTTPHandler) clientIP(r *http.Request) (string, error) {
	if h.RequestIP != nil {
		return h.RequestIP(r)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "unknown", nil
	}
	return host, nil
}

func strictBody(w http.ResponseWriter, r *http.Request, target any) error {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4096))
	if err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			return apiError("PAYLOAD_TOO_LARGE")
		}
		return apiError("INVALID_REQUEST")
	}
	if target == nil {
		if len(body) != 0 {
			return apiError("INVALID_REQUEST")
		}
		return nil
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return apiError("INVALID_REQUEST")
	}
	if !validObjectKeys(body, target) {
		return apiError("INVALID_REQUEST")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil {
		return apiError("INVALID_REQUEST")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return apiError("INVALID_REQUEST")
	}
	return nil
}

// encoding/json matches field names without case sensitivity and accepts
// duplicate keys. The public schema requires exact, unambiguous field names.
func validObjectKeys(body []byte, target any) bool {
	shape, err := json.Marshal(target)
	if err != nil {
		return false
	}
	var allowed map[string]json.RawMessage
	if json.Unmarshal(shape, &allowed) != nil {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return false
	}
	seen := make(map[string]bool)
	for decoder.More() {
		token, err := decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok || seen[key] {
			return false
		}
		if _, ok := allowed[key]; !ok {
			return false
		}
		seen[key] = true
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return false
		}
	}
	last, err := decoder.Token()
	if err != nil || last != json.Delim('}') {
		return false
	}
	var extra any
	return decoder.Decode(&extra) == io.EOF
}

func transactionID(r *http.Request) string {
	value := ""
	count := 0
	for _, cookie := range r.Cookies() {
		if cookie.Name == "__session" {
			value = cookie.Value
			count++
		}
	}
	if count != 1 || sessionCookieCount(r) != 1 || !validOpaque(value) {
		return ""
	}
	return value
}

func sessionCookieCount(r *http.Request) int {
	count := 0
	for _, header := range r.Header.Values("Cookie") {
		for _, part := range strings.Split(header, ";") {
			name, _, _ := strings.Cut(strings.TrimSpace(part), "=")
			if name == "__session" {
				count++
			}
		}
	}
	return count
}

func setTransactionCookie(w http.ResponseWriter, id string, age int) {
	http.SetCookie(w, &http.Cookie{Name: "__session", Value: id, Path: "/", MaxAge: age, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
}

func (h *HTTPHandler) callback(w http.ResponseWriter, r *http.Request, requestID string) {
	if len(r.URL.RawQuery) > 8192 {
		writeAPIError(w, 400, "INVALID_REQUEST", requestID)
		return
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeAPIError(w, 400, "INVALID_REQUEST", requestID)
		return
	}
	for key, values := range query {
		if len(values) != 1 || len(key) > 64 || len(values[0]) > 4096 || key == "returnUrl" {
			writeAPIError(w, 400, "INVALID_REQUEST", requestID)
			return
		}
	}
	code, state, providerError := query.Get("code"), query.Get("state"), query.Get("error")
	if state == "" || (code == "" && providerError == "") || (code != "" && providerError != "") {
		writeAPIError(w, 400, "INVALID_REQUEST", requestID)
		return
	}
	err = h.Auth.Callback(r.Context(), transactionID(r), state, code, providerError != "")
	if err == nil {
		http.Redirect(w, r, "/login/channel-confirm", http.StatusFound)
		return
	}
	redirectCode := "oauth_failed"
	switch errorCode(err) {
	case "OAUTH_TRANSACTION_REQUIRED", "OAUTH_TRANSACTION_PENDING", "OAUTH_TRANSACTION_CONSUMED", "OAUTH_TRANSACTION_CHANGED":
		redirectCode = "oauth_state_invalid"
	case "OAUTH_TRANSACTION_EXPIRED":
		redirectCode = "oauth_transaction_expired"
	case "OAUTH_SCOPE_INSUFFICIENT":
		redirectCode = "oauth_scope_insufficient"
	case "CHANNEL_UNAVAILABLE":
		redirectCode = "channel_unavailable"
	case "CHANNEL_AMBIGUOUS":
		redirectCode = "channel_ambiguous"
	}
	if providerError == "access_denied" {
		redirectCode = "oauth_denied"
	}
	http.Redirect(w, r, "/login?error="+redirectCode, http.StatusFound)
}

func statusFor(code string) int {
	switch code {
	case "AUTH_REQUIRED":
		return 401
	case "APP_CHECK_REQUIRED", "PRIVACY_RECONSENT_REQUIRED":
		return 403
	case "POLICY_VERSION_OUTDATED", "OAUTH_TRANSACTION_PENDING", "OAUTH_TRANSACTION_CONSUMED", "OAUTH_TRANSACTION_CHANGED", "WEB_ACCOUNT_REQUIRED":
		return 409
	case "OAUTH_TRANSACTION_EXPIRED":
		return 410
	case "PAYLOAD_TOO_LARGE":
		return 413
	case "RATE_LIMITED":
		return 429
	case "TEMPORARY_UNAVAILABLE":
		return 503
	case "INTERNAL_ERROR":
		return 500
	default:
		return 400
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		return
	}
}

func writeAPIError(w http.ResponseWriter, status int, code, requestID string) {
	writeJSON(w, status, struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"requestId"`
		} `json:"error"`
	}{Error: struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"requestId"`
	}{Code: code, Message: "request rejected", RequestID: requestID}})
}

func rateError(w http.ResponseWriter, retry int, id string) {
	if retry < 1 {
		retry = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(retry))
	writeAPIError(w, 429, "RATE_LIMITED", id)
}
