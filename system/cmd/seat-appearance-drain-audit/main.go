package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"google.golang.org/api/option"

	"app.modules/core/repository"
	"app.modules/core/utils"
)

type drainCounts struct {
	Seats       int
	MemberSeats int
}

func main() {
	if err := run(context.Background(), os.Args, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "seat-appearance-drain-audit:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) != 2 {
		return usageError()
	}
	expectedProjectID := strings.TrimSpace(args[1])
	if expectedProjectID == "" {
		return errors.New("expected GCP project ID is required")
	}

	utils.LoadEnv(".env")
	credentialFilePath := strings.TrimSpace(os.Getenv("CREDENTIAL_FILE_LOCATION"))
	if credentialFilePath == "" {
		return errors.New("CREDENTIAL_FILE_LOCATION is required")
	}
	//nolint:staticcheck // Operator-controlled credential file for this read-only audit.
	clientOption := option.WithCredentialsFile(credentialFilePath)

	actualProjectID, err := utils.GetGcpProjectID(ctx, clientOption)
	if err != nil {
		return fmt.Errorf("resolve GCP project ID: %w", err)
	}
	if actualProjectID != expectedProjectID {
		return fmt.Errorf(
			"GCP project mismatch: expected=%q credential=%q; refusing to audit",
			expectedProjectID,
			actualProjectID,
		)
	}

	repo, err := repository.NewFirestoreController(ctx, clientOption)
	if err != nil {
		return fmt.Errorf("initialize Firestore: %w", err)
	}
	defer func() {
		// A close failure does not invalidate counts already read and reported.
		if err := repo.FirestoreClient().Close(); err != nil {
			fmt.Fprintln(os.Stderr, "seat-appearance-drain-audit: close Firestore:", err)
		}
	}()

	seats, err := repo.ReadGeneralSeats(ctx)
	if err != nil {
		return fmt.Errorf("read all %s documents: %w", repository.SEATS, err)
	}
	memberSeats, err := repo.ReadMemberSeats(ctx)
	if err != nil {
		return fmt.Errorf("read all %s documents: %w", repository.MemberSeats, err)
	}

	if err := writeReport(out, drainCounts{
		Seats:       countV1Seats(seats),
		MemberSeats: countV1Seats(memberSeats),
	}); err != nil {
		return err
	}
	return nil
}

// countV1Seats treats every document other than exact schema-version 2 as
// undrained. This makes unknown future or malformed versions fail the gate.
func countV1Seats(seats []repository.SeatDoc) int {
	count := 0
	for _, seat := range seats {
		if seat.Appearance.SchemaVersion != utils.SeatAppearanceSchemaVersion {
			count++
		}
	}
	return count
}

func writeReport(out io.Writer, counts drainCounts) error {
	if _, err := fmt.Fprintf(out, "seats: %d\nmember-seats: %d\ntotal: %d\n", counts.Seats, counts.MemberSeats, counts.Seats+counts.MemberSeats); err != nil {
		return fmt.Errorf("write audit output: %w", err)
	}
	return nil
}

func usageError() error {
	return errors.New("usage: seat-appearance-drain-audit <expected-project-id>")
}
