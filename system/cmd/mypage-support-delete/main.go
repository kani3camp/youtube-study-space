// mypage-support-delete is an offline trusted-operator boundary. The default
// plan and --check-config modes do not read state. --mock exercises a fixed,
// independently seeded synthetic workflow. --execute always refuses because
// no live adapter, SDK bootstrap or credential discovery is linked here.
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
