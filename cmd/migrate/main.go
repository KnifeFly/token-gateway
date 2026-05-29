package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/KnifeFly/token-gateway/internal/bootstrap"
)

func main() {
	configPath := flag.String("config", "", "path to config file")
	direction := flag.String("direction", "up", "migration direction: up or down")
	flag.Parse()

	cfg, err := bootstrap.LoadConfig(*configPath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	if err := bootstrap.RunMigration(context.Background(), cfg, *direction); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "migration %s failed: %v\n", *direction, err)
		os.Exit(1)
	}
}
