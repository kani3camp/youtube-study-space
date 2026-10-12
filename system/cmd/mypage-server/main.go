// MyPage's Cloud Run entrypoint. --check-config validates offline; lifecycle
// tests inject synthetic bootstrap dependencies and never discover real ADC.
package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"cloud.google.com/go/firestore"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"

	"app.modules/core/mypage"
)

type configuration struct {
	server                                            mypage.ServerConfig
	signer, clientID, clientSecret, metadataKey, port string
}

func configurationFrom(get func(string) string) (configuration, error) {
	if get("MYPAGE_INFRASTRUCTURE_READY") != "true" {
		return configuration{}, errors.New("MyPage configuration unavailable")
	}
	return configurationValuesFrom(get)
}

// configurationValuesFrom is shared by startup and offline preparation. The
// infrastructure declaration gates startup separately; it is never verified by
// this pure validation or promoted into evidence of readiness.
func configurationValuesFrom(get func(string) string) (configuration, error) {
	fail := func() (configuration, error) { return configuration{}, errors.New("MyPage configuration unavailable") }
	if get("FIRESTORE_EMULATOR_HOST") != "" || get("FIREBASE_AUTH_EMULATOR_HOST") != "" || get("GOOGLE_APPLICATION_CREDENTIALS") != "" {
		return fail()
	}
	environment := get("MYPAGE_ENVIRONMENT")
	region := get("MYPAGE_REGION")
	if (environment != "development" || region != "asia-southeast2") && (environment != "production" || region != "asia-northeast2") {
		return fail()
	}
	config := configuration{server: mypage.ServerConfig{Environment: environment, ProjectID: get("GOOGLE_CLOUD_PROJECT"), ProjectNumber: get("MYPAGE_PROJECT_NUMBER"), WebAppID: get("MYPAGE_WEB_APP_ID"), PublicOrigin: get("MYPAGE_PUBLIC_ORIGIN"), Policy: mypage.Policy{Privacy: get("MYPAGE_PRIVACY_VERSION"), Terms: get("MYPAGE_TERMS_VERSION")}}, signer: get("MYPAGE_SIGNER_EMAIL"), clientID: get("MYPAGE_OAUTH_CLIENT_ID"), clientSecret: get("MYPAGE_OAUTH_CLIENT_SECRET"), metadataKey: get("MYPAGE_YOUTUBE_API_KEY"), port: get("PORT")}
	if config.port == "" {
		config.port = "8080"
	}
	port, portErr := strconv.Atoi(config.port)
	if portErr != nil || port < 1 || port > 65535 || config.server.ValidateRuntime(config.signer) != nil {
		return fail()
	}

	if _, err := mypage.NewGoogleYouTubeOAuth(config.clientID, config.clientSecret, config.server.PublicOrigin, nil); err != nil {
		return fail()
	}
	if _, err := mypage.NewAppCheckVerifier(config.server.ProjectNumber, config.server.WebAppID, nil, time.Now); err != nil {
		return fail()
	}
	if config.metadataKey != "" {
		if _, err := mypage.NewPublicYouTubeMetadata(config.metadataKey, nil); err != nil {
			return fail()
		}
	}
	return config, nil
}

// Only the three external bootstrap operations are replaceable in tests. The
// configuration gate, lazy provider/handler assembly and cleanup remain shared
// with production startup.
type bootstrapDependencies struct {
	credentials func(context.Context) (*google.Credentials, error)
	firebase    func(context.Context, configuration, *google.Credentials) (mypage.FirebaseAuthClient, error)
	database    func(context.Context, configuration, *google.Credentials) (*firestore.Client, func(), error)
}

func run(ctx context.Context) error {
	deps := bootstrapDependencies{
		credentials: func(ctx context.Context) (*google.Credentials, error) {
			return google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
		},
		firebase: func(ctx context.Context, config configuration, credentials *google.Credentials) (mypage.FirebaseAuthClient, error) {
			return mypage.NewKeylessFirebaseClient(ctx, config.server.ProjectID, config.signer, credentials, nil)
		},
		database: func(ctx context.Context, config configuration, credentials *google.Credentials) (*firestore.Client, func(), error) {
			database, err := firestore.NewClient(ctx, config.server.ProjectID, option.WithCredentials(credentials))
			if err != nil {
				return nil, nil, errors.New("MyPage database bootstrap unavailable")
			}
			return database, func() {
				database.Close() //nolint:errcheck // Shutdown errors must not expose credential/dependency detail.
			}, nil
		},
	}
	return runWith(ctx, os.Getenv, deps, func(ctx context.Context, server *http.Server) error {
		return serveHTTP(ctx, server, server.ListenAndServe, 10*time.Second)
	})
}

func runWith(ctx context.Context, get func(string) string, deps bootstrapDependencies, serve func(context.Context, *http.Server) error) error {
	config, err := configurationFrom(get)
	if err != nil {
		return err
	}
	bootstrapCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if bootstrapCtx.Err() != nil {
		return errors.New("MyPage keyless bootstrap unavailable")
	}
	credentials, err := deps.credentials(bootstrapCtx)
	if err != nil || bootstrapCtx.Err() != nil {
		return errors.New("MyPage keyless bootstrap unavailable")
	}
	sdk, err := deps.firebase(bootstrapCtx, config, credentials)
	if err != nil || bootstrapCtx.Err() != nil {
		return errors.New("MyPage keyless bootstrap unavailable")
	}
	database, closeDatabase, err := deps.database(bootstrapCtx, config, credentials)
	if err != nil {
		return errors.New("MyPage database bootstrap unavailable")
	}
	defer closeDatabase()
	if bootstrapCtx.Err() != nil {
		return errors.New("MyPage database bootstrap unavailable")
	}
	provider, err := mypage.NewGoogleYouTubeOAuth(config.clientID, config.clientSecret, config.server.PublicOrigin, nil)
	if err != nil {
		return errors.New("MyPage provider unavailable")
	}
	var metadata mypage.PublicMetadataReader
	if config.metadataKey != "" {
		metadata, err = mypage.NewPublicYouTubeMetadata(config.metadataKey, nil)
		if err != nil {
			return errors.New("MyPage metadata provider unavailable")
		}
	}
	handler, err := mypage.NewMyPageServer(config.server, mypage.ServerDependencies{Firestore: database, Firebase: sdk, OAuth: provider, PublicMetadata: metadata})
	if err != nil || bootstrapCtx.Err() != nil {
		return errors.New("MyPage handler unavailable")
	}
	server := &http.Server{Addr: ":" + config.port, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 25 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384, ErrorLog: log.New(io.Discard, "", 0)}
	return serve(ctx, server)
}

func serveHTTP(ctx context.Context, server *http.Server, serve func() error, shutdownTimeout time.Duration) error {
	done := make(chan struct{})
	shutdownDone := make(chan error, 1)
	go func() {
		select {
		case <-ctx.Done():
			shutdownCtx, stop := context.WithTimeout(context.Background(), shutdownTimeout)
			defer stop()
			err := server.Shutdown(shutdownCtx)
			if err != nil {
				// Terminate remaining connections before releasing the database.
				server.Close() //nolint:errcheck // Report only the fixed HTTP failure; never expose connection/dependency details.
			}
			shutdownDone <- err
		case <-done:
			shutdownDone <- nil
		}
	}()
	serveErr := serve()
	close(done)
	shutdownErr := <-shutdownDone
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		// An accept failure can leave previously accepted connections running.
		server.Close() //nolint:errcheck // Return the fixed HTTP diagnostic without dependency details.
		return errors.New("MyPage HTTP server unavailable")
	}
	if shutdownErr != nil {
		return errors.New("MyPage HTTP server unavailable")
	}
	return nil
}

func main() {
	os.Exit(command(os.Args[1:], os.Getenv, os.Stdout, os.Stderr, func() error {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		return run(ctx)
	}))
}
