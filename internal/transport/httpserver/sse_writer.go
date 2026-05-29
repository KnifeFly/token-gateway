package httpserver

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/KnifeFly/token-gateway/internal/provider/relay"
)

// writeStream writes a provider stream to the client.
func writeStream(ctx context.Context, w http.ResponseWriter, status int, stream relay.ProviderStream) {
	if status == 0 {
		status = http.StatusOK
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(status)

	flusher, _ := w.(http.Flusher)
	defer func() { _ = stream.Close() }()

	for {
		chunk, err := stream.Recv(ctx)
		if len(chunk) > 0 {
			if _, writeErr := w.Write(chunk); writeErr != nil {
				reportDownstreamError(stream, writeErr)
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				reportDownstreamError(stream, err)
			}
			return
		}
	}
}

func reportDownstreamError(stream relay.ProviderStream, err error) {
	reporter, ok := stream.(relay.DownstreamErrorReporter)
	if !ok {
		return
	}
	reporter.ReportDownstreamError(err)
}
