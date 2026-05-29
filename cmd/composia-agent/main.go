package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"forgejo.alexma.top/alexma233/composia/internal/app/agent"
	"forgejo.alexma.top/alexma233/composia/internal/platform/configpath"
)

var agentDefaultConfigPaths = []string{"/etc/composia/agent/config.yaml", "./config.yaml"}

func main() {
	configPath := flag.String("config", "", "agent config path")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	resolvedConfigPath, err := configpath.Resolve(*configPath, agentDefaultConfigPaths, "agent")
	if err != nil {
		log.Printf("composia agent failed: %v", err)
		stop()
		os.Exit(1)
	}
	if err := agent.Run(ctx, resolvedConfigPath); err != nil {
		log.Printf("composia agent failed: %v", err)
		stop()
		os.Exit(1)
	}
	stop()
}
