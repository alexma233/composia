//go:build unix

package controller

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func watchControllerReloadSignals(ctx context.Context, requests chan<- reloadRequest) func() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGHUP)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-signals:
				if err := requestControllerReload(ctx, requests); err != nil {
					log.Printf("controller config reload failed: %v", err)
				}
			}
		}
	}()
	return func() {
		signal.Stop(signals)
	}
}
