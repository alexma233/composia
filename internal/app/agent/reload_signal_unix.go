//go:build unix

package agent

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func watchAgentReloadSignals(ctx context.Context, requests chan<- agentReloadRequest) func() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGHUP)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-signals:
				if err := requestAgentReload(ctx, requests); err != nil {
					log.Printf("agent config reload failed: %v", err)
				}
			}
		}
	}()
	return func() {
		signal.Stop(signals)
	}
}
