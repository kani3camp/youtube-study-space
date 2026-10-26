package mypage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"reflect"
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
	Auth        *AuthService
	BFF         *BFF
	Maintenance MaintenanceReader
	Intake      PrivacyIntake
	History     *HistoryService
	Verifier    RequestVerifier
	// One fixed HTTPS origin per environment. Never derive it from request
	// headers; actual Hosting/proxy host forwarding must be verified at deploy.
	PublicOrigin string
	// Fixed backend host observed for Hosting rewrites. Forwarded headers are
	// routing metadata, never proof of identity or a replacement for Auth/App Check.
	HostingBackendHost string
	// A deployment may inject an extractor only after verifying its actual
	// trusted proxy chain. Default ignores all client forwarding headers.
	RequestIP func(*http.Request) (string, error)
	limiter   processLimiter
}

var routeMethods = map[string]string{
	"/api/public/mypage-status": "GET",
	"/api/auth/youtube/start":   "POST", "/api/auth/youtube/callback": "GET",
	"/api/auth/youtube/channel": "GET", "/api/auth/youtube/confirm": "POST",
	"/api/auth/session/complete": "POST", "/api/mypage": "GET",
	"/api/mypage/history":   "GET",
	"/api/privacy/requests": "POST", "/api/privacy/requests/status": "POST",
}

var maintenanceExemptRoutes = map[string]bool{
	"/api/public/mypage-status":    true,
	"/api/privacy/requests":        true,
	"/api/privacy/requests/status": true,
	"/api/auth/youtube/start":      true, // AuthService distinguishes login from support.
	"/api/auth/youtube/callback":   true, // AuthService reads the server-owned transaction purpose.
	"/api/auth/youtube/channel":    true,
	"/api/auth/youtube/confirm":    true,
}

type PrivacyIntake interface {
	Create(context.Context, string, string, SupportPurpose, string, time.Time) (PrivacyIntakeReceipt, error)
	Status(context.Context, string, string) (PrivacyRequestStatus, error)
}

func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID, err := opaque()
	if err != nil {
		requestID = "unavailable"
	}
	w.Header().Set("Cache-Control", "private, no-store")
	if r.URL.Path == "/api/public/mypage-status" {
		w.Header().Set("Cache-Control", "no-store")
	}
	w.Header().Set("X-Request-Id", requestID)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; base-uri 'none'; frame-ancestors 'none'")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	method, known := routeMethods[r.URL.Path]
	historyRoute := r.URL.Path == "/api/mypage/history"
	if historyRoute && h.History == nil {
		known = false
	}
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
	if !callback && !historyRoute && r.URL.RawQuery != "" {
		writeAPIError(w, 400, "INVALID_REQUEST", requestID)
		return
	}
	if r.Method == http.MethodGet && (r.ContentLength > 0 || len(r.TransferEncoding) > 0) {
		writeAPIError(w, 400, "INVALID_REQUEST", requestID)
		return
	}
	publicStatus := r.URL.Path == "/api/public/mypage-status"
	if (publicStatus && h.Maintenance == nil) || (!publicStatus && (h.Auth == nil || h.Verifier == nil || h.Auth.Now == nil)) {
		writeAPIError(w, 503, "TEMPORARY_UNAVAILABLE", requestID)
		return
	}
	if err := h.checkRequestOrigin(r); err != nil {
		code := errorCode(err)
		writeAPIError(w, statusFor(code), code, requestID)
		return
	}
	if publicStatus {
		ip, err := h.clientIP(r)
		if err != nil {
			writeAPIError(w, 400, "INVALID_REQUEST", requestID)
			return
		}
		if ok, retry := h.limiter.allow("maintenance-status:"+ip, 120, 30, time.Now().UTC()); !ok {
			rateError(w, retry, requestID)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		status, err := h.Maintenance.PublicStatus(ctx)
		if err != nil {
			writeAPIError(w, http.StatusServiceUnavailable, "TEMPORARY_UNAVAILABLE", requestID)
			return
		}
		writeJSON(w, http.StatusOK, status)
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
	var intake struct {
		SubmissionKey string         `json:"submissionKey"`
		Purpose       SupportPurpose `json:"purpose"`
		Body          string         `json:"body"`
	}
	var statusRequest struct {
		RequestRef string `json:"requestRef"`
	}
	if r.Method == http.MethodPost {
		var target any
		switch r.URL.Path {
		case "/api/auth/youtube/start":
			target = &start
		case "/api/auth/youtube/confirm":
			target = &confirmation
		case "/api/privacy/requests":
			target = &intake
		case "/api/privacy/requests/status":
			target = &statusRequest
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
	privacyRoute := strings.HasPrefix(r.URL.Path, "/api/privacy/requests")
	if privacyRoute {
		if h.Intake == nil {
			writeAPIError(w, 503, "TEMPORARY_UNAVAILABLE", requestID)
			return
		}
		if ok, retry := h.limiter.allow("privacy-pre:"+ip, 6, 3, now); !ok {
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
	authenticated := r.URL.Path == "/api/mypage" || historyRoute || r.URL.Path == "/api/auth/session/complete" || privacyRoute
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
		if privacyRoute {
			if ok, retry := h.limiter.allow("privacy-uid:"+identity.UID, 2, 2, now); !ok {
				rateError(w, retry, requestID)
				return
			}
		}
		if r.URL.Path == "/api/mypage" {
			if ok, retry := h.limiter.allow("mypage:"+identity.UID, 10, 10, now); !ok {
				rateError(w, retry, requestID)
				return
			}
		}
	}
	// Authentication and App Check identify the caller before the global gate.
	// SAC/account/policy and all personal-data reads remain after this point.
	if !maintenanceExemptRoutes[r.URL.Path] {
		if h.Maintenance != nil {
			if err := h.Maintenance.Check(ctx); err != nil {
				writeAPIError(w, statusFor(errorCode(err)), errorCode(err), requestID)
				return
			}
		}
	}
	if authenticated {
		if !privacyRoute {
			checkpoint, accessErr := readAccess(ctx, h.Auth.Access, identity.UID, "")
			if accessErr != nil {
				if h.BFF != nil {
					h.BFF.Invalidate(identity.UID)
				}
				writeAPIError(w, statusFor(errorCode(accessErr)), errorCode(accessErr), requestID)
				return
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
			account.AccessCheckpoint = checkpoint
			if err := checkPolicy(account, h.Auth.Policy); err != nil {
				if h.BFF != nil {
					h.BFF.Invalidate(identity.UID)
				}
				writeAPIError(w, statusFor(errorCode(err)), errorCode(err), requestID)
				return
			}
		}
	}
	switch r.URL.Path {
	case "/api/mypage/history":
		q, err := parseHistoryQuery(r.URL.RawQuery)
		if err != nil {
			writeAPIError(w, statusFor(errorCode(err)), errorCode(err), requestID)
			return
		}
		keys := r.Header.Values("Idempotency-Key")
		if len(keys) != 1 {
			writeAPIError(w, 400, "INVALID_REQUEST", requestID)
			return
		}
		page, err := h.History.Page(ctx, identity.UID, account, q, keys[0])
		if err != nil {
			var wait *HistoryWaitError
			if errors.As(err, &wait) {
				delay := max(1, int(time.Until(wait.ResetAt).Seconds())+1)
				w.Header().Set("Retry-After", strconv.Itoa(delay))
				w.Header().Set("X-Quota-Reset", wait.ResetAt.UTC().Format(time.RFC3339))
				code := "RATE_LIMITED"
				if wait.Daily {
					code = "ANALYSIS_BUDGET_EXHAUSTED"
				}
				writeAPIError(w, 429, code, requestID)
				return
			}
			writeAPIError(w, statusFor(errorCode(err)), errorCode(err), requestID)
			return
		}
		if !h.writePersonalJSON(w, ctx, 200, page, requestID, identity.UID, page.MaintenanceRevision) {
			return
		}
	case "/api/privacy/requests":
		response, err := h.Intake.Create(ctx, identity.UID, intake.SubmissionKey, intake.Purpose, intake.Body, now)
		if err != nil {
			code := errorCode(err)
			writeAPIError(w, statusFor(code), code, requestID)
			return
		}
		writeJSON(w, 200, response)
	case "/api/privacy/requests/status":
		response, err := h.Intake.Status(ctx, identity.UID, statusRequest.RequestRef)
		if err != nil {
			code := errorCode(err)
			writeAPIError(w, statusFor(code), code, requestID)
			return
		}
		writeJSON(w, 200, response)
	case "/api/auth/youtube/start":
		response, id, err := h.Auth.Start(ctx, transactionID(r), start)
		if err != nil {
			writeAPIError(w, statusFor(errorCode(err)), errorCode(err), requestID)
			return
		}
		if start.SupportChallenge == nil && h.Maintenance != nil {
			revision, err := h.Maintenance.CheckAuthoritativeRevision(ctx)
			if err != nil || response.MaintenanceRevision < 1 || revision != response.MaintenanceRevision {
				if err == nil {
					err = apiError("TEMPORARY_UNAVAILABLE")
				}
				if failErr := h.Auth.Store.Fail(ctx, id); failErr != nil {
					err = errors.Join(err, fmt.Errorf("fail maintenance OAuth transaction: %w", failErr))
				}
				writeAPIError(w, statusFor(errorCode(err)), errorCode(err), requestID)
				return
			}
		}
		setTransactionCookie(w, id, 600)
		writeJSON(w, 200, response)
	case "/api/auth/youtube/channel":
		response, err := h.Auth.Channel(ctx, transactionID(r))
		if err != nil {
			writeAPIError(w, statusFor(errorCode(err)), errorCode(err), requestID)
			return
		}
		if response.Purpose == "login" {
			if !h.writePersonalJSON(w, ctx, 200, response, requestID, "", response.MaintenanceRevision) {
				return
			}
		} else {
			writeJSON(w, 200, response)
		}
	case "/api/auth/youtube/confirm":
		response, err := h.Auth.ConfirmResult(ctx, transactionID(r), confirmation.ConfirmationRef)
		if err != nil {
			code := errorCode(err)
			if code == "TEMPORARY_UNAVAILABLE" || code == "MAINTENANCE_ACTIVE" || code == "OAUTH_TRANSACTION_CONSUMED" {
				setTransactionCookie(w, "", -1)
			}
			writeAPIError(w, statusFor(code), code, requestID)
			return
		}
		setTransactionCookie(w, "", -1)
		if response.Purpose == "login" {
			if !h.writePersonalJSON(w, ctx, 200, response, requestID, "", response.MaintenanceRevision) {
				return
			}
		} else {
			writeJSON(w, 200, response)
		}
	case "/api/auth/session/complete":
		maintenanceRevision := int64(0)
		operationContext := ctx
		if h.Maintenance != nil {
			maintenanceRevision, err = h.Maintenance.CheckAuthoritativeRevision(ctx)
			if err != nil {
				writeAPIError(w, statusFor(errorCode(err)), errorCode(err), requestID)
				return
			}
			operationContext = withMaintenanceRevision(ctx, maintenanceRevision)
		}
		if err := h.Auth.Store.CompleteSession(operationContext, identity.UID, account.AccessCheckpoint, h.Auth.Policy, now); err != nil {
			code := errorCode(err)
			if code == "INTERNAL_ERROR" {
				code = "TEMPORARY_UNAVAILABLE"
			}
			writeAPIError(w, statusFor(code), code, requestID)
			return
		}
		if h.Maintenance != nil {
			revision, err := h.Maintenance.CheckAuthoritativeRevision(ctx)
			if err != nil || revision != maintenanceRevision {
				if err == nil {
					err = apiError("TEMPORARY_UNAVAILABLE")
				}
				writeAPIError(w, statusFor(errorCode(err)), errorCode(err), requestID)
				return
			}
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
		if !h.writePersonalJSON(w, ctx, 200, response, requestID, identity.UID, response.MaintenanceRevision) {
			return
		}
	}
}

func (h *HTTPHandler) writePersonalJSON(w http.ResponseWriter, ctx context.Context, status int, value any, requestID, uid string, expectedMaintenanceRevision int64) bool {
	if h.Maintenance == nil {
		writeJSON(w, status, value)
		return true
	}
	revision, err := h.Maintenance.CheckAuthoritativeRevision(ctx)
	if err != nil || expectedMaintenanceRevision < 1 || revision != expectedMaintenanceRevision {
		if err == nil {
			err = apiError("TEMPORARY_UNAVAILABLE")
		}
		if uid != "" && h.BFF != nil {
			h.BFF.Invalidate(uid)
		}
		writeAPIError(w, statusFor(errorCode(err)), errorCode(err), requestID)
		return false
	}
	writeJSON(w, status, value)
	return true
}

func (h *HTTPHandler) checkRequestOrigin(r *http.Request) error {
	configured, err := url.Parse(h.PublicOrigin)
	if err != nil || configured.Scheme != "https" || configured.Host == "" || configured.User != nil || configured.Path != "" || configured.RawQuery != "" || configured.Fragment != "" {
		return apiError("TEMPORARY_UNAVAILABLE")
	}
	requestHost := r.Host
	if !strings.EqualFold(requestHost, configured.Host) {
		forwardedHosts, forwardedProtocols := r.Header.Values("X-Forwarded-Host"), r.Header.Values("X-Forwarded-Proto")
		if h.HostingBackendHost == "" || !validHostingBackendHost(h.HostingBackendHost) || !strings.EqualFold(requestHost, h.HostingBackendHost) ||
			len(forwardedHosts) != 1 || !strings.EqualFold(forwardedHosts[0], configured.Host) ||
			len(forwardedProtocols) != 1 || forwardedProtocols[0] != "https" {
			return apiError("INVALID_REQUEST")
		}
	}
	if r.URL.Host != "" && !strings.EqualFold(r.URL.Host, configured.Host) {
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
	shape := reflect.TypeOf(target)
	if shape == nil || shape.Kind() != reflect.Pointer || shape.Elem().Kind() != reflect.Struct {
		return false
	}
	shape = shape.Elem()
	allowed := make(map[string]bool)
	for i := 0; i < shape.NumField(); i++ {
		name, _, _ := strings.Cut(shape.Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" {
			allowed[name] = true
		}
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
		if !allowed[key] {
			return false
		}
		seen[key] = true
		var value json.RawMessage
		if decoder.Decode(&value) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
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
			if strings.TrimSpace(name) == "__session" {
				count++
			}
		}
	}
	return count
}

func setTransactionCookie(w http.ResponseWriter, id string, age int) {
	http.SetCookie(w, &http.Cookie{Name: "__session", Value: id, Path: "/", MaxAge: age, HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
}

func oauthRedirectCode(err error, providerError string) string {
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
	return redirectCode
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
	purpose, err := h.Auth.CallbackWithPurpose(r.Context(), transactionID(r), state, code, providerError != "")
	if err == nil {
		http.Redirect(w, r, "/login/channel-confirm", http.StatusFound)
		return
	}
	redirectCode := oauthRedirectCode(err, providerError)
	if purpose == "support" {
		setTransactionCookie(w, "", -1)
		http.Redirect(w, r, "/contact?supportError="+url.QueryEscape(redirectCode), http.StatusFound)
		return
	}
	if errorCode(err) == "MAINTENANCE_ACTIVE" {
		setTransactionCookie(w, "", -1)
		http.Redirect(w, r, "/maintenance", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/login?error="+redirectCode, http.StatusFound)
}

func statusFor(code string) int {
	switch code {
	case "FEATURE_UNAVAILABLE":
		return 404
	case "HISTORY_NOT_READY", "OPERATION_ALREADY_CONSUMED":
		return 409
	case "HISTORY_LIMIT_EXCEEDED":
		return 413
	case "ANALYSIS_BUDGET_EXHAUSTED":
		return 429
	case "AUTH_REQUIRED":
		return 401
	case "APP_CHECK_REQUIRED", "PRIVACY_RECONSENT_REQUIRED", "SERVICE_ACCESS_RESTRICTED", "DATA_DELETION_IN_PROGRESS":
		return 403
	case "POLICY_VERSION_OUTDATED", "OAUTH_TRANSACTION_PENDING", "OAUTH_TRANSACTION_CONSUMED", "OAUTH_TRANSACTION_CHANGED", "WEB_ACCOUNT_REQUIRED", "INTAKE_KEY_CONFLICT":
		return 409
	case "OAUTH_TRANSACTION_EXPIRED":
		return 410
	case "PAYLOAD_TOO_LARGE":
		return 413
	case "RATE_LIMITED":
		return 429
	case "MAINTENANCE_ACTIVE", "TEMPORARY_UNAVAILABLE":
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
