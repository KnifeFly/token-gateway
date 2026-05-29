package claude

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/provider/relay"
)

func TestMockMessageStream(t *testing.T) {
	response, err := NewAdapter(nil).Relay(context.Background(), relay.ChannelConfig{BaseURL: "mock://claude"}, relay.Request{
		CanonicalAPI: "claude.messages",
		PublicModel:  "claude-3-5-sonnet-latest",
		RawBody:      []byte(`{"model":"claude-3-5-sonnet-latest","stream":true,"messages":[{"role":"user","content":"hi"}]}`),
		Stream:       true,
	})
	if err != nil {
		t.Fatalf("Relay() error = %v", err)
	}
	if response.Stream == nil {
		t.Fatal("missing stream")
	}
	chunk, err := response.Stream.Recv(context.Background())
	if err != nil {
		t.Fatalf("Recv() error = %v", err)
	}
	if !strings.Contains(string(chunk), "content_block_delta") {
		t.Fatalf("chunk = %q", chunk)
	}
	_, _ = response.Stream.Recv(context.Background())
	if _, err := response.Stream.Recv(context.Background()); err != io.EOF {
		t.Fatalf("Recv() err = %v, want EOF", err)
	}
}

func TestParseUsageMessageStartShape(t *testing.T) {
	usage := parseUsage([]byte(`{"type":"message_start","message":{"usage":{"input_tokens":5,"output_tokens":1}}}`))
	if usage.InputTokens != 5 || usage.OutputTokens != 1 || usage.TotalTokens != 6 {
		t.Fatalf("usage = %#v", usage)
	}
}
