package mypage

import (
	"context"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"app.modules/core/serviceaccess"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

// NewKeylessFirebaseClient accepts an existing ADC token source, never a key
// file. Provisioning the named signer and granting iam.serviceAccounts.signBlob
// is a separate human-approved infrastructure operation, not this constructor.
func NewKeylessFirebaseClient(ctx context.Context, projectID, signerEmail string, credentials *google.Credentials, client *http.Client) (*auth.Client, error) {
	if validateKeylessSigner(projectID, signerEmail) != nil || credentials == nil || credentials.TokenSource == nil || len(credentials.JSON) != 0 || (credentials.ProjectID != "" && credentials.ProjectID != projectID) || os.Getenv("FIREBASE_AUTH_EMULATOR_HOST") != "" || ctx.Err() != nil {
		return nil, apiError("TEMPORARY_UNAVAILABLE")
	}

	opts := []option.ClientOption{option.WithCredentials(credentials)}
	if client != nil {
		bounded := *client
		if bounded.Timeout <= 0 || bounded.Timeout > 5*time.Second {
			bounded.Timeout = 5 * time.Second
		}
		bounded.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		opts = append(opts, option.WithHTTPClient(&bounded))
	}
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID, ServiceAccountID: signerEmail}, opts...)
	if err != nil {
		return nil, apiError("TEMPORARY_UNAVAILABLE")
	}
	authClient, err := app.Auth(ctx)
	if err != nil {
		return nil, apiError("TEMPORARY_UNAVAILABLE")
	}
	return authClient, nil
}

type ServerConfig struct {
	Environment, ProjectID, ProjectNumber, WebAppID, PublicOrigin string
	Policy                                                        Policy
	HostingBackendHost                                            string
	// Default off. Enable only after the reply operator and no-session
	// exception contact are operational and D02/D03 are resolved.
	EnablePrivacyIntake bool
	PrivacyIntakeSecret []byte
	// Supply only verified historical coverage; nil means unavailable history.
	Coverage []Interval
}

// Only a literal Cloud Run service hostname is configurable, with no scheme,
// port, path, wildcard or user-controlled suffix. The caller pins the observed
// service hostname; the header never selects a backend or a redirect target.
var hostingBackendHostPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,61}\.a\.run\.app$`)

func validHostingBackendHost(host string) bool {
	return hostingBackendHostPattern.MatchString(host)
}

// ValidateRuntime checks fixed environment/project/app/origin and signer before
// any ADC discovery. The readiness decision remains a human infrastructure gate.
func (config ServerConfig) ValidateRuntime(signerEmail string) error {
	if err := config.validate(); err != nil {
		return err
	}
	return validateKeylessSigner(config.ProjectID, signerEmail)
}

func validateKeylessSigner(projectID, signerEmail string) error {
	signerName := strings.TrimSuffix(signerEmail, "@"+projectID+".iam.gserviceaccount.com")
	if !firebaseProjectID.MatchString(projectID) || !firebaseProjectID.MatchString(signerName) || signerEmail == signerName {
		return apiError("TEMPORARY_UNAVAILABLE")
	}
	return nil
}

func (config ServerConfig) validate() error {
	if !validSupportEnvironment(config.Environment) || !firebaseProjectID.MatchString(config.ProjectID) || config.Policy.Privacy == "" || config.Policy.Terms == "" {
		return apiError("TEMPORARY_UNAVAILABLE")
	}
	if config.HostingBackendHost != "" && !validHostingBackendHost(config.HostingBackendHost) {
		return apiError("TEMPORARY_UNAVAILABLE")
	}
	if config.EnablePrivacyIntake && len(config.PrivacyIntakeSecret) < 32 {
		return apiError("TEMPORARY_UNAVAILABLE")
	}
	if _, err := NewGoogleYouTubeOAuth("validation-only", "validation-only", config.PublicOrigin, nil); err != nil {
		return err
	}
	_, err := NewAppCheckVerifier(config.ProjectNumber, config.WebAppID, nil, time.Now)
	return err
}

type ServerDependencies struct {
	Firestore      *firestore.Client
	Firebase       FirebaseAuthClient
	OAuth          YouTubeOAuth
	Metadata       MetadataRefresher
	PublicMetadata PublicMetadataReader
	AppCheckHTTP   *http.Client
	Now            func() time.Time
	// Optional DEV-only, no-identity cost observation hook.
	ReadObserver func(SnapshotReadObservation)
	// Domain-separated cursor keys derive from the existing server OAuth
	// secret. Never supplied by the browser or written to policy documents.
	HistorySecret  []byte
	HistoryTierFor func(context.Context, string, WebAccount) (HistoryTier, error)
}

// NewMyPageServer constructs the handler without bootstrap IO,
// service-account creation, secret access, listen/deploy, or notifications.
func NewMyPageServer(config ServerConfig, deps ServerDependencies) (*HTTPHandler, error) {
	if config.validate() != nil || deps.Firestore == nil || deps.Firebase == nil || deps.OAuth == nil {
		return nil, apiError("TEMPORARY_UNAVAILABLE")
	}
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	appCheck, err := NewAppCheckVerifier(config.ProjectNumber, config.WebAppID, deps.AppCheckHTTP, now)
	if err != nil {
		return nil, err
	}
	boundary := &FirebaseBoundary{Client: deps.Firebase, AppCheck: appCheck, ProjectID: config.ProjectID, Now: now}
	maintenance := &MaintenanceGate{Client: deps.Firestore, Environment: config.Environment, Now: now, TTL: 5 * time.Second}
	store := &FirestoreAuthStore{Client: deps.Firestore, Environment: config.Environment}
	access := &serviceaccess.FirestoreStore{Client: deps.Firestore}
	metadata := deps.Metadata
	if metadata == nil && deps.PublicMetadata != nil {
		metadata = &AccountMetadataRefresh{Access: access, Provider: deps.PublicMetadata, Store: store, Policy: config.Policy, Now: now}
	}

	handler := &HTTPHandler{
		PublicOrigin: config.PublicOrigin, HostingBackendHost: config.HostingBackendHost, Verifier: boundary,
		Maintenance: maintenance,
		Auth:        &AuthService{Access: access, Store: store, Maintenance: maintenance, Provider: deps.OAuth, Minter: boundary, Policy: config.Policy, Now: now, Support: &FirestoreSupportStore{Client: deps.Firestore, Environment: config.Environment}},
		BFF:         &BFF{Access: access, Maintenance: maintenance, Reader: &FirestoreSnapshotReader{Client: deps.Firestore, Coverage: append([]Interval(nil), config.Coverage...), Observe: deps.ReadObserver}, Environment: config.Environment, Now: now, Metadata: metadata},
	}
	if config.EnablePrivacyIntake {
		handler.Intake = &FirestorePrivacyIntake{Client: deps.Firestore, Environment: config.Environment, IntakeSecret: append([]byte(nil), config.PrivacyIntakeSecret...)}
	}
	if config.Environment == "development" && len(deps.HistorySecret) > 0 {
		cursor, err := newHistoryCipher(deps.HistorySecret, config.Environment)
		if err != nil {
			return nil, err
		}
		handler.History = &HistoryService{Client: deps.Firestore, Access: access, Maintenance: maintenance, Environment: config.Environment, Cipher: cursor, TierFor: deps.HistoryTierFor, Now: now}
	}
	return handler, nil
}
