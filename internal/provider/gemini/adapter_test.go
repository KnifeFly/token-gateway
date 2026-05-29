package gemini

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/KnifeFly/token-gateway/internal/provider/relay"
)

func TestMockGenerateContentStream(t *testing.T) {
	response, err := NewAdapter(nil).Relay(context.Background(), relay.ChannelConfig{BaseURL: "mock://gemini"}, relay.Request{
		CanonicalAPI:  "gemini.generate_content",
		PublicModel:   "gemini-2.5-flash",
		UpstreamModel: "gemini-2.5-flash",
		RawBody:       []byte(`{"contents":[{"parts":[{"text":"hi"}]}]}`),
		Stream:        true,
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
	if !strings.Contains(string(chunk), "M3 Gemini mock response") {
		t.Fatalf("chunk = %q", chunk)
	}
	_, _ = response.Stream.Recv(context.Background())
	if _, err := response.Stream.Recv(context.Background()); err != io.EOF {
		t.Fatalf("Recv() err = %v, want EOF", err)
	}
}

func TestParseUsageMetadataShape(t *testing.T) {
	usage := parseUsage([]byte(`{"usageMetadata":{"promptTokenCount":7,"candidatesTokenCount":9,"totalTokenCount":16}}`))
	if usage.InputTokens != 7 || usage.OutputTokens != 9 || usage.TotalTokens != 16 {
		t.Fatalf("usage = %#v", usage)
	}
}
