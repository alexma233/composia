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
	os.Exit(run())
}

func run() int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	var received atomic.Int32
	go func() {
		sig := <-signals
		switch sig {
		case os.Interrupt:
			received.Store(2)
		case syscall.SIGTERM:
			received.Store(15)
		}
		cancel()
	}()

	if err := cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		if errors.Is(err, context.Canceled) && received.Load() != 0 {
			return 128 + int(received.Load())
		}
		return 1
	}
	return 0
}
