// The Cloud Run Job changes the MyPage planned-maintenance switch.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"cloud.google.com/go/firestore"

	"app.modules/core/mypage"
)

type changeFunc func(context.Context, bool) (mypage.MaintenanceControl, bool, error)

func parseEnabled(args []string) (bool, error) {
	flags := flag.NewFlagSet("mypage-maintenance-operator", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	enabled := flags.String("enabled", "", "true or false")
	if flags.Parse(args) != nil || flags.NArg() != 0 || (*enabled != "true" && *enabled != "false") {
		return false, errors.New("enabled must be true or false")
	}
	return *enabled == "true", nil
}

func run(ctx context.Context, args []string, output io.Writer, change changeFunc) error {
	enabled, err := parseEnabled(args)
	if err != nil {
		return err
	}
	value, changed, err := change(ctx, enabled)
	if err != nil {
		return fmt.Errorf("set maintenance: %w", err)
	}
	if err := json.NewEncoder(output).Encode(struct {
		Enabled   bool      `json:"enabled"`
		Changed   bool      `json:"changed"`
		UpdatedAt time.Time `json:"updatedAt"`
	}{value.Enabled, changed, value.UpdatedAt}); err != nil {
		return fmt.Errorf("write result: %w", err)
	}
	return nil
}

func main() {
	if _, err := parseEnabled(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	env := os.Getenv("MYPAGE_ENVIRONMENT")
	project := map[string]string{"development": "test-youtube-study-space", "production": "youtube-study-space"}[env]
	if project == "" || project != os.Getenv("GOOGLE_CLOUD_PROJECT") {
		fmt.Fprintln(os.Stderr, "invalid environment or project")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := firestore.NewClient(ctx, project)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect Firestore:", err)
		os.Exit(1)
	}
	defer func() {
		if err := client.Close(); err != nil {
			fmt.Fprintln(os.Stderr, "close Firestore:", err)
		}
	}()
	if err = run(ctx, os.Args[1:], os.Stdout, func(ctx context.Context, enabled bool) (mypage.MaintenanceControl, bool, error) {
		return mypage.SetMaintenance(ctx, client, enabled)
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
