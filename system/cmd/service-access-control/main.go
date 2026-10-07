// service-access-control is a trusted operator CLI for moderation only. It
// provides no public admin API, privacy execution, migration, deletion or TTL.
// The default plan and --check-config paths never discover ADC or call an SDK.
package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"

	"app.modules/core/serviceaccess"
)

func keylessCredentialsValid(credentials *google.Credentials, project string) bool {
	return credentials != nil && credentials.TokenSource != nil && len(credentials.JSON) == 0 && (credentials.ProjectID == "" || credentials.ProjectID == project)
}

func keylessBootstrap(ctx context.Context, selected target) (ports, error) {
	credentials, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform", "https://www.googleapis.com/auth/identitytoolkit")
	if err != nil || !keylessCredentialsValid(credentials, selected.ProjectID) {
		return ports{}, errors.New("keyless bootstrap unavailable")
	}
	client, err := firestore.NewClient(ctx, selected.ProjectID, option.WithCredentials(credentials))
	if err != nil {
		return ports{}, errors.New("database bootstrap unavailable")
	}
	return ports{
		Store: &serviceaccess.FirestoreStore{Client: client},
		Close: client.Close,
		Revoke: func(ctx context.Context, channel string) error {
			// Firebase initialization is deliberately lazy: its failure must leave
			// the already committed moderation guard active and report partial failure.
			app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: selected.ProjectID}, option.WithCredentials(credentials))
			if err != nil {
				return errors.New("revocation unavailable")
			}
			sdk, err := app.Auth(ctx)
			if err != nil {
				return errors.New("revocation unavailable")
			}
			if err := sdk.RevokeRefreshTokens(ctx, channel); err != nil && !auth.IsUserNotFound(err) {
				return errors.New("revocation unavailable")
			}
			return nil
		},
	}, nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	os.Exit(command(ctx, os.Args[1:], os.Getenv, os.Stdin, os.Stdout, os.Stderr, keylessBootstrap, time.Now))
}
