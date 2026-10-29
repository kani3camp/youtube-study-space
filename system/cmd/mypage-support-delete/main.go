// mypage-support-deleteは、信頼済み運用担当者向けのオフライン処理である。
// 既定のplanと--check-configは状態を読まない。
// --mockは独立して用意した固定の合成ワークフローを検証する。
// 実環境用アダプター・SDK初期化・認証情報探索を接続していないため、--executeは常に拒否する。
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"app.modules/core/supportdelete"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	os.Exit(command(ctx, os.Args[1:], os.Getenv, os.Stdin, os.Stdout, os.Stderr, supportdelete.RunMock))
}
