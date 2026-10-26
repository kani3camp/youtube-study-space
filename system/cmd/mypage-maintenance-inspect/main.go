// The maintenance inspection service is a fixed-digest, IAM-authenticated
// readback boundary. Its caller can invoke this one service but never receives
// Firestore credentials or an arbitrary document selector.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"app.modules/core/mypage"

	"cloud.google.com/go/firestore"
)

var errInspect = errors.New("maintenance inspection is unavailable")

type inspectionReader func(context.Context) (mypage.MaintenanceInspection, error)

func inspectionHandler(read inspectionReader) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.Method != http.MethodGet || r.URL.Path != "/" || r.URL.RawQuery != "" || r.ContentLength != 0 || len(r.TransferEncoding) != 0 {
			http.Error(w, `{"error":{"code":"INVALID_REQUEST"}}`, http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		receipt, err := read(ctx)
		if err != nil {
			http.Error(w, `{"error":{"code":"TEMPORARY_UNAVAILABLE"}}`, http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(receipt); err != nil {
			return
		}
	})
}

func configuration(args []string, get func(string) string) (string, string, string, error) {
	if len(args) != 0 || get("GOOGLE_APPLICATION_CREDENTIALS") != "" || get("FIRESTORE_EMULATOR_HOST") != "" || get("FIREBASE_AUTH_EMULATOR_HOST") != "" {
		return "", "", "", errInspect
	}
	environment := get("MYPAGE_ENVIRONMENT")
	project := get("GOOGLE_CLOUD_PROJECT")
	if map[string]string{"development": "test-youtube-study-space", "production": "youtube-study-space"}[environment] != project || get("PORT") != "8080" {
		return "", "", "", errInspect
	}
	return project, environment, get("PORT"), nil
}

func main() {
	project, environment, port, err := configuration(os.Args[1:], os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, errInspect)
		os.Exit(1)
	}
	client, err := firestore.NewClient(context.Background(), project)
	if err != nil {
		fmt.Fprintln(os.Stderr, errInspect)
		os.Exit(1)
	}
	defer func() {
		if err := client.Close(); err != nil {
			fmt.Fprintln(os.Stderr, errInspect)
		}
	}()
	server := &http.Server{
		Addr: ":" + port,
		Handler: inspectionHandler(func(ctx context.Context) (mypage.MaintenanceInspection, error) {
			return mypage.InspectMaintenanceControl(ctx, client, environment)
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      12 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    8192,
	}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, errInspect)
		os.Exit(1)
	}
}
