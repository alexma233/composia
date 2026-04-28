package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"forgejo.alexma.top/alexma233/composia/internal/app/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
