package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	role := flag.String("role", "main", "process role: main or agent")
	configPath := flag.String("config", "./config.yaml", "path to the config file")
	flag.Parse()

	switch *role {
	case "main", "agent":
		fmt.Printf("composia starting: role=%s config=%s\n", *role, *configPath)
	default:
		fmt.Fprintf(os.Stderr, "invalid role %q, expected main or agent\n", *role)
		os.Exit(2)
	}
}
