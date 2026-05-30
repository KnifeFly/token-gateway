package builtin

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/KnifeFly/token-gateway/internal/dataplane/plugin"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// Callback controls async task callback URLs before they enter the outbox.
type Callback struct{}

type callbackConfig struct {
	Mode         string   `json:"mode"`
	CallbackURL  string   `json:"callback_url"`
	AllowedHosts []string `json:"allowed_hosts"`
}

func (Callback) Name() string {
	return "callback"
}

func (Callback) Phase() plugin.Phase {
	return plugin.PhasePrePrompt
}

func (Callback) Validate(config json.RawMessage) error {
	var cfg callbackConfig
	if err := decodeConfig(config, &cfg); err != nil {
		return err
	}
	if cfg.Mode != "" && cfg.Mode != "allow" && cfg.Mode != "require" && cfg.Mode != "disable" && cfg.Mode != "override" {
		return apperr.InvalidArgument("callback mode is not supported")
	}
	if cfg.CallbackURL != "" {
		_, err := parseCallbackURL(cfg.CallbackURL)
		return err
	}
	return nil
}

func (Callback) Execute(_ context.Context, input plugin.Input) (plugin.Result, error) {
	var cfg callbackConfig
	if err := decodeConfig(input.Config, &cfg); err != nil {
		return plugin.Result{}, err
	}
	if input.State == nil || input.State.Parsed.Media == nil {
		return plugin.Result{Action: plugin.ActionAllow}, nil
	}
	mode := cfg.Mode
	if mode == "" {
		mode = "allow"
	}
	callbackURL := strings.TrimSpace(input.State.Parsed.Media.CallbackURL)
	switch mode {
	case "disable":
		input.State.Parsed.Media.CallbackURL = ""
		return callbackAudit("disabled"), nil
	case "override":
		if cfg.CallbackURL == "" {
			return plugin.Result{}, apperr.InvalidArgument("callback_url is required for override mode")
		}
		callbackURL = cfg.CallbackURL
	case "require":
		if callbackURL == "" {
			return plugin.Result{Action: plugin.ActionDeny, Message: "callback_url is required"}, nil
		}
	}
	if callbackURL == "" {
		return plugin.Result{Action: plugin.ActionAllow}, nil
	}
	parsed, err := parseCallbackURL(callbackURL)
	if err != nil {
		return plugin.Result{}, err
	}
	if len(cfg.AllowedHosts) > 0 && !hostAllowed(parsed.Hostname(), cfg.AllowedHosts) {
		return plugin.Result{Action: plugin.ActionDeny, Message: "callback host is not allowed"}, nil
	}
	input.State.Parsed.Media.CallbackURL = parsed.String()
	return callbackAudit(mode), nil
}

func callbackAudit(action string) plugin.Result {
	return plugin.Result{
		Action: plugin.ActionAllow,
		AuditFields: map[string]string{
			"plugin": "callback",
			"action": action,
		},
		Metadata: map[string]string{
			"plugin.callback.action": action,
		},
	}
}

func parseCallbackURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, apperr.InvalidArgument("callback_url is invalid")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil, apperr.InvalidArgument("callback_url scheme is not supported")
	}
	return parsed, nil
}

func hostAllowed(host string, allowed []string) bool {
	for _, value := range allowed {
		if strings.EqualFold(strings.TrimSpace(value), host) {
			return true
		}
	}
	return false
}
