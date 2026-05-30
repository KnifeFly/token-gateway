package bootstrap

import (
	"context"
	"testing"
)

func TestNewConfigdAppInitializesWithoutExternalDependencies(t *testing.T) {
	cfg := DefaultConfig()
	app, err := NewConfigdApp(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewConfigdApp() error = %v", err)
	}
	if app == nil || app.Logger() == nil {
		t.Fatalf("app = %#v", app)
	}
	if err := app.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
