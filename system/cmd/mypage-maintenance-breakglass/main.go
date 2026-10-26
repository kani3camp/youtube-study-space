// MyPage maintenance mode's break-glass Cloud Run Job entrypoint. It is a
// separate fixed command and Job; GitHub provenance cannot be selected here.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"app.modules/core/mypage"
	maintenanceoperator "app.modules/internal/mypagemaintenanceoperator"

	"cloud.google.com/go/firestore"
)

func main() {
	err := maintenanceoperator.Run(context.Background(), os.Args[1:], os.Stdout, os.Getenv, time.Now, func(ctx context.Context, project string) (*firestore.Client, error) {
		return firestore.NewClient(ctx, project)
	}, mypage.MaintenanceAuthorityBreakGlass)
	if err != nil {
		fmt.Fprintln(os.Stderr, maintenanceoperator.ErrOperator)
		os.Exit(1)
	}
}
