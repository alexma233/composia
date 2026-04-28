package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"forgejo.alexma.top/alexma233/composia/internal/agent"
	"forgejo.alexma.top/alexma233/composia/internal/cli"
	"forgejo.alexma.top/alexma233/composia/internal/controller"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	command := os.Args[1]
	args := os.Args[2:]

	var err error
	switch command {
	case "controller":
		configPath, parseErr := parseConfigFlag(args)
		if parseErr != nil {
			err = parseErr
			break
		}
		err = controller.Run(ctx, configPath)
	case "agent":
		configPath, parseErr := parseConfigFlag(args)
		if parseErr != nil {
			err = parseErr
			break
		}
		err = agent.Run(ctx, configPath)
	default:
		err = cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	}

	if err != nil {
		if command == "controller" || command == "agent" {
			log.Printf("composia failed: %v", err)
		} else {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		os.Exit(1)
	}
}

func parseConfigFlag(args []string) (string, error) {
	configPath := "./config.yaml"
	for index := 0; index < len(args); index++ {
		current := args[index]
		switch current {
		case "-config", "--config":
			if index+1 >= len(args) {
				return "", fmt.Errorf("missing value for %s", current)
			}
			configPath = args[index+1]
			index++
		default:
			return "", fmt.Errorf("unknown argument %q", current)
		}
	}
	return configPath, nil
}

func usage() {
	cli.PrintUsage(os.Stderr)
}
