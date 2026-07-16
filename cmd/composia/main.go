package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"forgejo.alexma.top/alexma233/composia/internal/app/cli"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	var received atomic.Int32
	go func() {
		if sig, ok := (<-signals).(syscall.Signal); ok {
			received.Store(int32(sig))
		}
		cancel()
	}()

	if err := cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		cancel()
		if errors.Is(err, context.Canceled) && received.Load() != 0 {
			os.Exit(128 + int(received.Load()))
		}
		os.Exit(1)
	}
	cancel()
}
