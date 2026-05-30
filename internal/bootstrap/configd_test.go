package bootstrap

import (
	"context"
	"testing"
	"time"
)

func TestConfigdAppRestartsWithPublishOnStart(t *testing.T) {
	for i := 0; i < 2; i++ {
		cfg := DefaultConfig()
		cfg.Configd.Addr = "127.0.0.1:0"
		cfg.Configd.ShutdownTimeout = Duration{time.Second}
		cfg.Configd.PublishOnStart = true
		cfg.Telemetry.MetricsEnabled = false

		app, err := NewConfigdApp(context.Background(), cfg)
		if err != nil {
			t.Fatalf("NewConfigdApp(%d) error = %v", i, err)
		}
		runCtx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error, 1)
		go func() {
			errCh <- app.Run(runCtx)
		}()

		time.Sleep(20 * time.Millisecond)
		cancel()

		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("Run(%d) error = %v", i, err)
			}
		case <-time.After(time.Second):
			t.Fatalf("Run(%d) did not stop after context cancellation", i)
		}
	}
}
