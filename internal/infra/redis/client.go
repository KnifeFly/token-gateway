package redis

import (
	"context"
	"crypto/tls"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Config controls Redis setup.
type Config struct {
	Enabled      bool
	Addr         string
	Username     string
	Password     string
	DB           int
	TLS          bool
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// Status describes dependency readiness.
type Status struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// Client wraps a go-redis client and disabled-state behavior.
type Client struct {
	enabled bool
	client  *goredis.Client
	logger  *slog.Logger
}

// New creates a Redis client. It does not ping during construction.
func New(_ context.Context, cfg Config, logger *slog.Logger) (*Client, error) {
	c := &Client{enabled: cfg.Enabled, logger: logger}
	if !cfg.Enabled {
		return c, nil
	}
	opts := &goredis.Options{
		Addr:         cfg.Addr,
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}
	if cfg.TLS {
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	c.client = goredis.NewClient(opts)
	return c, nil
}

// Enabled reports whether Redis is configured.
func (c *Client) Enabled() bool {
	return c != nil && c.enabled
}

// Raw returns the underlying go-redis client.
func (c *Client) Raw() *goredis.Client {
	if c == nil {
		return nil
	}
	return c.client
}

// Ping reports readiness.
func (c *Client) Ping(ctx context.Context) Status {
	if c == nil || !c.enabled {
		return Status{Name: "redis", Status: "skipped"}
	}
	if err := c.client.Ping(ctx).Err(); err != nil {
		return Status{Name: "redis", Status: "unavailable", Error: err.Error()}
	}
	return Status{Name: "redis", Status: "ready"}
}

// Close closes the Redis client.
func (c *Client) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}
