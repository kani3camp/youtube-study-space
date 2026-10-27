// The maintenance bootstrap check is a fixed, read-only Cloud Run Job. It is
// invoked by deployment identities before planning a guarded serving revision.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"app.modules/core/mypage"

	"cloud.google.com/go/firestore"
)

var errBootstrap = errors.New("maintenance bootstrap is not a verified initialized control")

type clientFactory func(context.Context, string) (*firestore.Client, error)

func run(ctx context.Context, args []string, get func(string) string, connect clientFactory) (_ mypage.MaintenanceInspection, resultErr error) {
	if len(args) != 0 || get("GOOGLE_APPLICATION_CREDENTIALS") != "" || get("FIRESTORE_EMULATOR_HOST") != "" || get("FIREBASE_AUTH_EMULATOR_HOST") != "" {
		return mypage.MaintenanceInspection{}, errBootstrap
	}
	environment := get("MYPAGE_ENVIRONMENT")
	project := get("GOOGLE_CLOUD_PROJECT")
	if map[string]string{"development": "test-youtube-study-space", "production": "youtube-study-space"}[environment] != project {
		return mypage.MaintenanceInspection{}, errBootstrap
	}
	client, err := connect(ctx, project)
	if err != nil {
		return mypage.MaintenanceInspection{}, errBootstrap
	}
	defer func() {
		if client.Close() != nil {
			resultErr = errBootstrap
		}
	}()
	checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	receipt, err := mypage.InspectMaintenanceControl(checkCtx, client, environment)
	if err != nil {
		return mypage.MaintenanceInspection{}, errBootstrap
	}
	return receipt, nil
}

func main() {
	receipt, err := run(context.Background(), os.Args[1:], os.Getenv, func(ctx context.Context, project string) (*firestore.Client, error) {
		return firestore.NewClient(ctx, project)
	})
	if err != nil || json.NewEncoder(os.Stdout).Encode(receipt) != nil {
		fmt.Fprintln(os.Stderr, errBootstrap)
		os.Exit(1)
	}
}
