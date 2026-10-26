// The maintenance bootstrap check is a fixed, read-only Cloud Run Job. It is
// invoked by deployment identities before planning a guarded serving revision.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"app.modules/core/mypage"

	"cloud.google.com/go/firestore"
)

var errBootstrap = errors.New("maintenance bootstrap is not a verified initialized control")

type clientFactory func(context.Context, string) (*firestore.Client, error)

func run(ctx context.Context, args []string, get func(string) string, connect clientFactory) (resultErr error) {
	if len(args) != 0 || get("GOOGLE_APPLICATION_CREDENTIALS") != "" || get("FIRESTORE_EMULATOR_HOST") != "" || get("FIREBASE_AUTH_EMULATOR_HOST") != "" {
		return errBootstrap
	}
	environment := get("MYPAGE_ENVIRONMENT")
	project := get("GOOGLE_CLOUD_PROJECT")
	if map[string]string{"development": "test-youtube-study-space", "production": "youtube-study-space"}[environment] != project {
		return errBootstrap
	}
	client, err := connect(ctx, project)
	if err != nil {
		return errBootstrap
	}
	defer func() {
		if client.Close() != nil {
			resultErr = errBootstrap
		}
	}()
	checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if mypage.RequireMaintenanceInitialized(checkCtx, client, environment) != nil {
		return errBootstrap
	}
	return nil
}

func main() {
	if run(context.Background(), os.Args[1:], os.Getenv, func(ctx context.Context, project string) (*firestore.Client, error) {
		return firestore.NewClient(ctx, project)
	}) != nil {
		fmt.Fprintln(os.Stderr, errBootstrap)
		os.Exit(1)
	}
	fmt.Println("maintenance bootstrap control verified")
}
