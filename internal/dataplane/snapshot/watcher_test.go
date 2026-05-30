package snapshot

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/KnifeFly/token-gateway/internal/controlplane/admin"
	cpsnapshot "github.com/KnifeFly/token-gateway/internal/controlplane/snapshot"
	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
)

func TestWatcherPollsPublishAndRollbackSnapshots(t *testing.T) {
	ctx := context.Background()
	repo := admin.NewMemoryRepository()
	service := admin.NewService(repo, admin.NewCredentialCodec("secret"), nil)
	seedWatcherSnapshotConfig(t, ctx, service, "gpt-4o-mini")

	publisher := cpsnapshot.NewPublisher(repo, cpsnapshot.NewBuilder(repo))
	first, err := publisher.Publish(ctx)
	if err != nil {
		t.Fatalf("Publish(first) error = %v", err)
	}
	store := NewStore(nil)
	watcher := NewWatcher(cpsnapshot.NewActiveProvider(repo), store, nil, time.Hour, nil)
	if err := watcher.Poll(ctx); err != nil {
		t.Fatalf("Poll(first) error = %v", err)
	}
	assertCurrentSnapshotModel(t, store, first.Version, "gpt-4o-mini", true)

	upsertWatcherSnapshotModel(t, ctx, service, "gpt-4.1-mini")
	second, err := publisher.Publish(ctx)
	if err != nil {
		t.Fatalf("Publish(second) error = %v", err)
	}
	if second.Version == first.Version {
		t.Fatal("snapshot version did not advance")
	}
	if err := watcher.Poll(ctx); err != nil {
		t.Fatalf("Poll(second) error = %v", err)
	}
	assertCurrentSnapshotModel(t, store, second.Version, "gpt-4.1-mini", true)

	rolledBack, err := publisher.Rollback(ctx)
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if rolledBack.Version != first.Version {
		t.Fatalf("rollback version = %q, want %q", rolledBack.Version, first.Version)
	}
	if err := watcher.Poll(ctx); err != nil {
		t.Fatalf("Poll(rollback) error = %v", err)
	}
	assertCurrentSnapshotModel(t, store, first.Version, "gpt-4.1-mini", false)
}

func TestWatcherKeepsCurrentSnapshotWhenConfigdUnavailableAndStalePolicyApplies(t *testing.T) {
	current, err := Build(cpsnapshot.RuntimeSnapshot{
		Version:   "last-known-good",
		CreatedAt: time.Now().UTC(),
		Models: []cpsnapshot.ModelRuntime{{
			PublicModel: "gpt-4o-mini",
			Protocol:    string(engine.ProtocolNativeOpenAI),
			Capability:  "chat",
			Enabled:     true,
		}},
	})
	if err != nil {
		t.Fatalf("Build(current) error = %v", err)
	}
	store := NewStore(current)
	watcher := NewWatcher(failingActiveProvider{}, store, nil, time.Hour, nil)
	if err := watcher.Poll(context.Background()); err == nil {
		t.Fatal("Poll() succeeded, want configd error")
	}
	assertCurrentSnapshotModel(t, store, "last-known-good", "gpt-4o-mini", true)

	stale, err := Build(cpsnapshot.RuntimeSnapshot{
		Version:   "hard-stale",
		CreatedAt: time.Now().UTC().Add(-time.Hour),
		Models: []cpsnapshot.ModelRuntime{{
			PublicModel: "gpt-4o-mini",
			Protocol:    string(engine.ProtocolNativeOpenAI),
			Capability:  "chat",
			Enabled:     true,
		}},
	})
	if err != nil {
		t.Fatalf("Build(stale) error = %v", err)
	}
	if err := store.Replace(stale); err != nil {
		t.Fatalf("Replace(stale) error = %v", err)
	}
	provider := NewProvider(store, WithStalePolicy(StalePolicy{
		SoftTTL: time.Minute,
		HardTTL: 2 * time.Minute,
	}))
	if err := provider.Attach(context.Background(), &engine.RequestState{Internal: map[string]any{}}); err == nil {
		t.Fatal("Attach() succeeded, want hard stale error")
	}
}

func TestFallbackActiveProviderUsesFallbackWhenPrimaryUnavailable(t *testing.T) {
	createdAt := time.Now().UTC()
	fallback := staticActiveProvider{runtime: &cpsnapshot.RuntimeSnapshot{
		Version:   "fallback",
		CreatedAt: createdAt,
		Models: []cpsnapshot.ModelRuntime{{
			PublicModel: "gpt-4o-mini",
			Protocol:    string(engine.ProtocolNativeOpenAI),
			Capability:  "chat",
			Enabled:     true,
		}},
	}}
	provider := NewFallbackActiveProvider(failingActiveProvider{}, fallback)
	runtime, ok, err := provider.ActiveRuntimeSnapshot(context.Background())
	if err != nil {
		t.Fatalf("ActiveRuntimeSnapshot() error = %v", err)
	}
	if !ok || runtime.Version != "fallback" {
		t.Fatalf("runtime = %#v ok = %v", runtime, ok)
	}
}

func TestWatcherPollsOnSnapshotEvent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repo := admin.NewMemoryRepository()
	service := admin.NewService(repo, admin.NewCredentialCodec("secret"), nil)
	seedWatcherSnapshotConfig(t, ctx, service, "gpt-4o-mini")
	publisher := cpsnapshot.NewPublisher(repo, cpsnapshot.NewBuilder(repo))
	first, err := publisher.Publish(ctx)
	if err != nil {
		t.Fatalf("Publish(first) error = %v", err)
	}
	store := NewStore(nil)
	events := newManualSnapshotEvents()
	watcher := NewWatcher(cpsnapshot.NewActiveProvider(repo), store, nil, time.Hour, nil, WithEventSource(events))
	go watcher.Start(ctx)
	waitForSnapshot(t, store, first.Version)

	upsertWatcherSnapshotModel(t, ctx, service, "gpt-4.1-mini")
	second, err := publisher.Publish(ctx)
	if err != nil {
		t.Fatalf("Publish(second) error = %v", err)
	}
	events.notify()
	waitForSnapshot(t, store, second.Version)
}

type failingActiveProvider struct{}

func (failingActiveProvider) ActiveRuntimeSnapshot(context.Context) (*cpsnapshot.RuntimeSnapshot, bool, error) {
	return nil, false, errors.New("configd unavailable")
}

type staticActiveProvider struct {
	runtime *cpsnapshot.RuntimeSnapshot
}

func (p staticActiveProvider) ActiveRuntimeSnapshot(context.Context) (*cpsnapshot.RuntimeSnapshot, bool, error) {
	if p.runtime == nil {
		return nil, false, nil
	}
	return p.runtime, true, nil
}

type manualSnapshotEvents struct {
	ch chan struct{}
}

func newManualSnapshotEvents() *manualSnapshotEvents {
	return &manualSnapshotEvents{ch: make(chan struct{}, 1)}
}

func (e *manualSnapshotEvents) SnapshotEvents(context.Context) (<-chan struct{}, func() error, error) {
	return e.ch, func() error { return nil }, nil
}

func (e *manualSnapshotEvents) notify() {
	select {
	case e.ch <- struct{}{}:
	default:
	}
}

func seedWatcherSnapshotConfig(t *testing.T, ctx context.Context, service *admin.Service, model string) {
	t.Helper()
	if _, err := service.CreateAPIKey(ctx, admin.APIKey{TenantID: "tenant", ProjectID: "project", PlaintextKey: "tg-test"}); err != nil {
		t.Fatalf("CreateAPIKey() error = %v", err)
	}
	upsertWatcherSnapshotModel(t, ctx, service, model)
}

func upsertWatcherSnapshotModel(t *testing.T, ctx context.Context, service *admin.Service, model string) {
	t.Helper()
	if _, err := service.UpsertModel(ctx, admin.ModelConfig{
		PublicModel: model,
		Protocol:    string(engine.ProtocolNativeOpenAI),
		Capability:  "chat",
		Enabled:     true,
	}); err != nil {
		t.Fatalf("UpsertModel(%s) error = %v", model, err)
	}
	if _, err := service.UpsertChannel(ctx, admin.ChannelConfig{
		ID:           "channel_1",
		ProviderType: "openai_compatible",
		BaseURL:      "mock://openai",
		Enabled:      true,
		Models:       []admin.ChannelModel{{PublicModel: model, UpstreamModel: model}},
	}); err != nil {
		t.Fatalf("UpsertChannel(%s) error = %v", model, err)
	}
	if _, err := service.UpsertRoute(ctx, admin.RoutePolicyConfig{
		PublicModel: model,
		Candidates:  []admin.RouteCandidate{{ChannelID: "channel_1", Priority: 1, Weight: 100}},
	}); err != nil {
		t.Fatalf("UpsertRoute(%s) error = %v", model, err)
	}
}

func assertCurrentSnapshotModel(t *testing.T, store *Store, version string, model string, want bool) {
	t.Helper()
	current, err := store.Current()
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if current.Ref().Version != version {
		t.Fatalf("current version = %q, want %q", current.Ref().Version, version)
	}
	_, ok := current.LookupModel(model)
	if ok != want {
		t.Fatalf("LookupModel(%q) ok = %v, want %v", model, ok, want)
	}
}

func waitForSnapshot(t *testing.T, store *Store, version string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		current, err := store.Current()
		if err == nil && current.Ref().Version == version {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	current, err := store.Current()
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	t.Fatalf("current version = %q, want %q", current.Ref().Version, version)
}
