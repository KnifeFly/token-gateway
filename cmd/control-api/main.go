package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/KnifeFly/token-gateway/internal/bootstrap"
	loginfra "github.com/KnifeFly/token-gateway/internal/infra/log"
)

func main() {
	configPath := flag.String("config", "", "path to config file")
	flag.Parse()

	cfg, err := bootstrap.LoadConfig(*configPath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	logger := loginfra.New(loginfra.Config{Level: cfg.Telemetry.LogLevel, Format: cfg.Telemetry.LogFormat}, os.Stdout)
	logger.Info("control-api is not implemented in M0", "service", cfg.Service.Name)
}
