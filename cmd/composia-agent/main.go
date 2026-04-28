package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"forgejo.alexma.top/alexma233/composia/internal/agent"
)

func main() {
	configPath := flag.String("config", "./config.yaml", "agent config path")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := agent.Run(ctx, *configPath); err != nil {
		log.Printf("composia agent failed: %v", err)
		os.Exit(1)
	}
}
