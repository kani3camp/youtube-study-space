package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"app.modules/core/repository"
)

func TestRunRequiresExpectedProjectIDBeforeCredentialAccess(t *testing.T) {
	t.Setenv("CREDENTIAL_FILE_LOCATION", "")

	err := run(context.Background(), []string{"seat-appearance-drain-audit", ""}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("run() error = nil, want project ID validation error")
	}
}

func TestRunRequiresExplicitTarget(t *testing.T) {
	err := run(context.Background(), []string{"seat-appearance-drain-audit"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("run() error = nil, want usage error")
	}
}

func TestRunRefusesFirestoreEmulatorHost(t *testing.T) {
	t.Setenv("FIRESTORE_EMULATOR_HOST", "localhost:8080")
	t.Setenv("CREDENTIAL_FILE_LOCATION", "")

	err := run(context.Background(), []string{"seat-appearance-drain-audit", "expected-project"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("run() error = nil, want emulator rejection")
	}
	if !strings.Contains(err.Error(), "FIRESTORE_EMULATOR_HOST") {
		t.Fatalf("run() error = %q, want FIRESTORE_EMULATOR_HOST rejection", err)
	}
}

func TestCountV1SeatsRequiresExactSchemaVersion2(t *testing.T) {
	seats := []repository.SeatDoc{
		{},
		{Appearance: repository.SeatAppearance{SchemaVersion: 1}},
		{Appearance: repository.SeatAppearance{SchemaVersion: 2}},
		{Appearance: repository.SeatAppearance{SchemaVersion: 3}},
	}

	if got, want := countV1Seats(seats), 3; got != want {
		t.Fatalf("countV1Seats() = %d, want %d", got, want)
	}
}

func TestWriteReportUsesGateFormat(t *testing.T) {
	var out bytes.Buffer
	if err := writeReport(&out, drainCounts{Seats: 2, MemberSeats: 3}); err != nil {
		t.Fatal(err)
	}

	const want = "seats: 2\nmember-seats: 3\ntotal: 5\n"
	if got := out.String(); got != want {
		t.Fatalf("writeReport() = %q, want %q", got, want)
	}
}
