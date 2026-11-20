// service-access-controlは、信頼済み運用担当者向けのモデレーション専用CLIである。
// 公開管理API・プライバシー処理の実行・移行・削除・TTLは提供しない。
// 既定のplanと--check-configではADC探索やSDK呼び出しを行わない。
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
			// Firebaseは意図的に遅延初期化する。失敗時もコミット済みのモデレーション保護を有効に保ち、
			// 部分失敗として報告する。
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
