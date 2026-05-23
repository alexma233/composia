package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"forgejo.alexma.top/alexma233/composia/internal/app/controller"
	"forgejo.alexma.top/alexma233/composia/internal/platform/configpath"
)

var controllerDefaultConfigPaths = []string{"/etc/composia/controller/config.yaml", "./config.yaml"}

func main() {
	configPath := flag.String("config", "", "controller config path")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	resolvedConfigPath, err := configpath.Resolve(*configPath, controllerDefaultConfigPaths, "controller")
	if err != nil {
		log.Printf("composia controller failed: %v", err)
		os.Exit(1)
	}
	if err := controller.Run(ctx, resolvedConfigPath); err != nil {
		log.Printf("composia controller failed: %v", err)
		os.Exit(1)
	}
}
