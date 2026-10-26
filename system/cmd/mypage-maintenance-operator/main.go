// MyPage maintenance mode's GitHub Cloud Run Job entrypoint. Its authority is
// compiled into this binary and cannot be selected through execution args.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"app.modules/core/mypage"
	maintenanceoperator "app.modules/internal/mypagemaintenanceoperator"

	"cloud.google.com/go/firestore"
)

var errOperator = maintenanceoperator.ErrOperator

type clientFactory func(context.Context, string) (*firestore.Client, error)

func run(ctx context.Context, args []string, output io.Writer, get func(string) string, now func() time.Time, connect clientFactory) error {
	if err := maintenanceoperator.Run(ctx, args, output, get, now, maintenanceoperator.ClientFactory(connect), mypage.MaintenanceAuthorityGitHub); err != nil {
		return fmt.Errorf("run maintenance operator: %w", err)
	}
	return nil
}

func main() {
	err := run(context.Background(), os.Args[1:], os.Stdout, os.Getenv, time.Now, func(ctx context.Context, project string) (*firestore.Client, error) {
		return firestore.NewClient(ctx, project)
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, errOperator)
		os.Exit(1)
	}
}
