package integrationtest

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"app.modules/core/repository"

	"google.golang.org/api/option"
)

const (
	expectedProjectID     = "demo-youtube-study-space-ci"
	expectedEmulatorHost  = "127.0.0.1"
	expectedEmulatorPort  = "8080"
	emulatorHostEnv       = "FIRESTORE_EMULATOR_HOST"
	emulatorProjectEnv    = "GCLOUD_PROJECT"
	googleProjectEnv      = "GOOGLE_CLOUD_PROJECT"
	firebaseProjectEnv    = "FIREBASE_PROJECT_ID"
	emulatorDatabaseName  = "(default)"
	emulatorResetPathBase = "/emulator/v1/projects/"
)

// RequireFirestoreEmulator prevents integration tests from accidentally using a real Firestore project.
func RequireFirestoreEmulator(t *testing.T) {
	t.Helper()

	hostPort := os.Getenv(emulatorHostEnv)
	if hostPort == "" {
		t.Fatal("FIRESTORE_EMULATOR_HOST must be set for Firestore integration tests")
	}

	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		t.Fatalf("invalid FIRESTORE_EMULATOR_HOST %q: %v", hostPort, err)
	}
	if !isLoopbackHost(host) {
		t.Fatalf("FIRESTORE_EMULATOR_HOST must point to a loopback address, got %q", hostPort)
	}
	if port != expectedEmulatorPort {
		t.Fatalf("FIRESTORE_EMULATOR_HOST must use port %s, got %q", expectedEmulatorPort, port)
	}

	projectID := ""
	for _, envName := range []string{emulatorProjectEnv, googleProjectEnv, firebaseProjectEnv} {
		value := os.Getenv(envName)
		if value == "" {
			continue
		}
		if projectID == "" {
			projectID = value
			continue
		}
		if value != projectID {
			t.Fatalf("Firestore emulator project ID environment variables disagree: %s=%q, %s=%q", envName, value, emulatorProjectEnv, projectID)
		}
	}
	if projectID == "" {
		t.Fatalf("one of %s, %s, or %s must be set to %q", emulatorProjectEnv, googleProjectEnv, firebaseProjectEnv, expectedProjectID)
	}
	if projectID != expectedProjectID {
		t.Fatalf("Firestore integration tests require project ID %q, got %q", expectedProjectID, projectID)
	}

	// firestore.DetectProjectID recognizes GOOGLE_CLOUD_PROJECT. Keep direct local
	// invocations safe even when only GCLOUD_PROJECT was supplied by the caller.
	if os.Getenv(googleProjectEnv) == "" {
		t.Setenv(googleProjectEnv, projectID)
	}
}

// NewFirestoreController creates a repository connected to the local Firestore Emulator.
func NewFirestoreController(t *testing.T) *repository.FirestoreControllerImplements {
	t.Helper()
	RequireFirestoreEmulator(t)

	client, err := repository.NewFirestoreController(context.Background(), option.WithoutAuthentication())
	if err != nil {
		t.Fatalf("create Firestore emulator client: %v", err)
	}
	t.Cleanup(func() {
		if err := client.FirestoreClient().Close(); err != nil {
			t.Errorf("close Firestore emulator client: %v", err)
		}
	})
	return client
}

// ResetFirestore removes every document from the emulator's default database.
func ResetFirestore(t *testing.T) {
	t.Helper()
	RequireFirestoreEmulator(t)

	hostPort := os.Getenv(emulatorHostEnv)
	endpoint := fmt.Sprintf(
		"http://%s%s%s/databases/%s/documents",
		hostPort,
		emulatorResetPathBase,
		url.PathEscape(expectedProjectID),
		emulatorDatabaseName,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		t.Fatalf("create Firestore emulator reset request: %v", err)
	}

	response, err := (&http.Client{}).Do(request)
	if err != nil {
		t.Fatalf("reset Firestore emulator data: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 4<<10))
		if readErr != nil {
			t.Fatalf("reset Firestore emulator data returned status %s and response body could not be read: %v", response.Status, readErr)
		}
		t.Fatalf("reset Firestore emulator data returned unexpected status %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback() && (ip.String() == expectedEmulatorHost || ip.String() == "::1")
}
