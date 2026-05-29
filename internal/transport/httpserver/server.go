package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

// Config controls the HTTP server.
type Config struct {
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	MaxHeaderBytes    int
}

// Server wraps net/http.Server.
type Server struct {
	http    *http.Server
	timeout time.Duration
	logger  *slog.Logger
}

// New creates a server.
func New(cfg Config, handler http.Handler, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		timeout: cfg.ShutdownTimeout,
		logger:  logger,
		http: &http.Server{
			Addr:              cfg.Addr,
			Handler:           handler,
			ReadHeaderTimeout: cfg.ReadHeaderTimeout,
			ReadTimeout:       cfg.ReadTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
			MaxHeaderBytes:    cfg.MaxHeaderBytes,
		},
	}
}

// Start runs the HTTP server.
func (s *Server) Start() error {
	s.logger.Info("http server starting", "addr", s.http.Addr)
	err := s.http.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	s.logger.Info("http server shutting down")
	return s.http.Shutdown(ctx)
}
