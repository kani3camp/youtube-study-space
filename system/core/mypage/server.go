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

// NewKeylessFirebaseClientは既存ADCのtoken sourceを受け取り、鍵ファイルは使わない。
// 指定署名者の構築とiam.serviceAccounts.signBlob付与は、
// 人が承認する別のインフラ操作であり、このコンストラクターでは行わない。
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
	// 完全性を検証した過去期間だけを渡す。nilは履歴が取得不能であることを表す。
	Coverage []Interval
}

// 設定できるのはCloud Runサービスのホスト名そのものだけとする。
// スキーム・ポート・パス・ワイルドカード・利用者指定の接尾辞は認めない。
// 呼び出し元が観測済みホスト名を固定し、ヘッダーでバックエンドやリダイレクト先を選ばない。
var hostingBackendHostPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,61}\.a\.run\.app$`)

func validHostingBackendHost(host string) bool {
	return hostingBackendHostPattern.MatchString(host)
}

// ValidateRuntimeはADC探索前に、固定の環境・プロジェクト・アプリ・オリジン・署名者を確認する。
// 準備完了の判断は、人による基盤確認として残す。
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
	// DEV専用で本人情報を含まない、任意の原価観測フック。
	ReadObserver func(SnapshotReadObservation)
}

// NewMyPageServerはハンドラーを構築するだけで、初期化時の入出力、
// サービスアカウント作成、シークレット取得、待ち受け・配備、通知は行わない。
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
	maintenance := &MaintenanceGate{Client: deps.Firestore}
	store := &FirestoreAuthStore{Client: deps.Firestore, MaintenanceEnabled: true}
	access := &serviceaccess.FirestoreStore{Client: deps.Firestore}
	metadata := deps.Metadata
	if metadata == nil && deps.PublicMetadata != nil {
		metadata = &AccountMetadataRefresh{Access: access, Provider: deps.PublicMetadata, Store: store, Policy: config.Policy, Now: now}
	}

	handler := &HTTPHandler{
		PublicOrigin: config.PublicOrigin, HostingBackendHost: config.HostingBackendHost, Verifier: boundary,
		Maintenance: maintenance,
		Auth:        &AuthService{Access: access, Store: store, Maintenance: maintenance, Provider: deps.OAuth, Minter: boundary, Policy: config.Policy, Now: now, Support: &FirestoreSupportStore{Client: deps.Firestore, Environment: config.Environment}},
		BFF:         &BFF{Access: access, Reader: &FirestoreSnapshotReader{Client: deps.Firestore, Coverage: append([]Interval(nil), config.Coverage...), Observe: deps.ReadObserver}, Environment: config.Environment, Now: now, Metadata: metadata},
	}
	return handler, nil
}
