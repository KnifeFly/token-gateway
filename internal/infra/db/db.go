package db

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	// Register the MySQL driver for sql.Open.
	_ "github.com/go-sql-driver/mysql"
)

// Config controls SQL database setup.
type Config struct {
	Enabled         bool
	Driver          string
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	MigrationsDir   string
}

// Status describes dependency readiness.
type Status struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// Client wraps sql.DB and disabled-state behavior.
type Client struct {
	enabled       bool
	db            *sql.DB
	migrationsDir string
	logger        *slog.Logger
}

// New creates a database client. It does not ping during construction.
func New(_ context.Context, cfg Config, logger *slog.Logger) (*Client, error) {
	c := &Client{enabled: cfg.Enabled, migrationsDir: cfg.MigrationsDir, logger: logger}
	if !cfg.Enabled {
		return c, nil
	}
	db, err := sql.Open(cfg.Driver, cfg.DSN)
	if err != nil {
		return nil, err
	}
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	c.db = db
	return c, nil
}

// Enabled reports whether the database dependency is configured.
func (c *Client) Enabled() bool {
	return c != nil && c.enabled
}

// DB returns the underlying sql.DB.
func (c *Client) DB() *sql.DB {
	if c == nil {
		return nil
	}
	return c.db
}

// Ping reports readiness.
func (c *Client) Ping(ctx context.Context) Status {
	if c == nil || !c.enabled {
		return Status{Name: "database", Status: "skipped"}
	}
	if err := c.db.PingContext(ctx); err != nil {
		return Status{Name: "database", Status: "unavailable", Error: err.Error()}
	}
	return Status{Name: "database", Status: "ready"}
}

// Close closes the database connection.
func (c *Client) Close() error {
	if c == nil || c.db == nil {
		return nil
	}
	return c.db.Close()
}

// MigrateUp applies pending migrations.
func (c *Client) MigrateUp(ctx context.Context) error {
	if c == nil || !c.enabled || c.db == nil {
		return ErrDisabled
	}
	return MigrateUp(ctx, c.db, c.migrationsDir)
}

// MigrateDown reverts the latest migration.
func (c *Client) MigrateDown(ctx context.Context) error {
	if c == nil || !c.enabled || c.db == nil {
		return ErrDisabled
	}
	return MigrateDown(ctx, c.db, c.migrationsDir)
}
