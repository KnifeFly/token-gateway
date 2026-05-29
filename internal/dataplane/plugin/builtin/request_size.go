package builtin

import (
	"context"
	"encoding/json"

	"github.com/KnifeFly/token-gateway/internal/dataplane/plugin"
)

// RequestSize rejects requests that exceed configured size limits.
type RequestSize struct{}

type requestSizeConfig struct {
	MaxHeaderBytes int64 `json:"max_header_bytes"`
	MaxBodyBytes   int64 `json:"max_body_bytes"`
	MaxFileBytes   int64 `json:"max_file_bytes"`
	MaxFiles       int   `json:"max_files"`
}

func (RequestSize) Name() string {
	return "request_size"
}

func (RequestSize) Phase() plugin.Phase {
	return plugin.PhasePreRequest
}

func (RequestSize) Validate(config json.RawMessage) error {
	var cfg requestSizeConfig
	return decodeConfig(config, &cfg)
}

func (RequestSize) Execute(_ context.Context, input plugin.Input) (plugin.Result, error) {
	var cfg requestSizeConfig
	if err := decodeConfig(input.Config, &cfg); err != nil {
		return plugin.Result{}, err
	}
	if input.State == nil {
		return plugin.Result{}, nil
	}
	if cfg.MaxHeaderBytes > 0 && headerBytes(input.State.Incoming.Header) > cfg.MaxHeaderBytes {
		return plugin.Result{Action: plugin.ActionDeny, Message: "request headers exceed size policy"}, nil
	}
	if cfg.MaxBodyBytes > 0 && input.State.Incoming.ContentLength > cfg.MaxBodyBytes {
		return plugin.Result{Action: plugin.ActionDeny, Message: "request body exceeds size policy"}, nil
	}
	if cfg.MaxFileBytes > 0 && input.State.Parsed.File != nil && input.State.Parsed.File.SizeBytes > cfg.MaxFileBytes {
		return plugin.Result{Action: plugin.ActionDeny, Message: "file exceeds size policy"}, nil
	}
	return plugin.Result{Action: plugin.ActionAllow}, nil
}

func headerBytes(header map[string][]string) int64 {
	var total int64
	for key, values := range header {
		total += int64(len(key))
		for _, value := range values {
			total += int64(len(value))
		}
	}
	return total
}
