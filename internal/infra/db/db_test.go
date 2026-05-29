package db

import (
	"context"
	"testing"
)

func TestDisabledClient(t *testing.T) {
	client, err := New(context.Background(), Config{Enabled: false}, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if client.Enabled() {
		t.Fatal("client should be disabled")
	}
	status := client.Ping(context.Background())
	if status.Status != "skipped" {
		t.Fatalf("status = %q", status.Status)
	}
	if err := client.MigrateUp(context.Background()); err != ErrDisabled {
		t.Fatalf("MigrateUp() error = %v", err)
	}
}
