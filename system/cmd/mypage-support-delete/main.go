// mypage-support-deleteは、信頼済み運用担当者向けのオフライン処理である。
// 既定のplanと--check-configは状態を読まない。
// --executeだけがowner-controlled ADCで現在storeへ接続する。未確認のRunbook Gateは保留する。
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	os.Exit(command(ctx, os.Args[1:], os.Getenv, os.Stdin, os.Stdout, os.Stderr, executeCurrent))
}
