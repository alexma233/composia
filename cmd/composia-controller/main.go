package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"forgejo.alexma.top/alexma233/composia/internal/controller"
)

func main() {
	configPath := flag.String("config", "./config.yaml", "controller config path")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := controller.Run(ctx, *configPath); err != nil {
		log.Printf("composia controller failed: %v", err)
		os.Exit(1)
	}
}
