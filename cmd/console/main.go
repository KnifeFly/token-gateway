package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/KnifeFly/token-gateway/internal/bootstrap"
)

func main() {
	configPath := flag.String("config", "", "path to config file")
	flag.Parse()

	cfg, err := bootstrap.LoadConfig(*configPath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := bootstrap.NewConsoleApp(ctx, cfg)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "initialize console: %v\n", err)
		os.Exit(1)
	}
	if err := app.Run(ctx); err != nil {
		app.Logger().Error("console stopped with error", "error", err)
		os.Exit(1)
	}
}
