package builtin

import (
	"context"
	"encoding/json"
	"net"
	"strings"

	"github.com/KnifeFly/token-gateway/internal/dataplane/plugin"
)

// IPAllowlist allows or denies callers by client IP without exposing IP labels.
type IPAllowlist struct{}

type ipAllowlistConfig struct {
	AllowCIDRs []string `json:"allow_cidrs"`
	DenyCIDRs  []string `json:"deny_cidrs"`
}

// Name returns the IP allowlist plugin name.
func (IPAllowlist) Name() string {
	return "ip_allowlist"
}

// Phase returns the pre-request IP allowlist phase.
func (IPAllowlist) Phase() plugin.Phase {
	return plugin.PhasePreRequest
}

// Validate verifies configured CIDR ranges.
func (IPAllowlist) Validate(config json.RawMessage) error {
	var cfg ipAllowlistConfig
	if err := decodeConfig(config, &cfg); err != nil {
		return err
	}
	_, err := compileCIDRs(append(cfg.AllowCIDRs, cfg.DenyCIDRs...))
	return err
}

// Execute enforces allow and deny CIDR policies for the request client IP.
func (IPAllowlist) Execute(_ context.Context, input plugin.Input) (plugin.Result, error) {
	var cfg ipAllowlistConfig
	if err := decodeConfig(input.Config, &cfg); err != nil {
		return plugin.Result{}, err
	}
	clientIP := parseClientIP("")
	if input.State != nil {
		clientIP = parseClientIP(input.State.ClientIP)
	}
	allowCIDRs, err := compileCIDRs(cfg.AllowCIDRs)
	if err != nil {
		return plugin.Result{}, err
	}
	denyCIDRs, err := compileCIDRs(cfg.DenyCIDRs)
	if err != nil {
		return plugin.Result{}, err
	}
	if clientIP == nil {
		return plugin.Result{Action: plugin.ActionDeny, Message: "client ip is required"}, nil
	}
	if ipInAnyCIDR(clientIP, denyCIDRs) {
		return plugin.Result{
			Action:  plugin.ActionDeny,
			Message: "client ip is not allowed",
			AuditFields: map[string]string{
				"plugin": "ip_allowlist",
				"action": "deny",
				"match":  "deny_cidr",
			},
		}, nil
	}
	if len(allowCIDRs) > 0 && !ipInAnyCIDR(clientIP, allowCIDRs) {
		return plugin.Result{
			Action:  plugin.ActionDeny,
			Message: "client ip is not allowed",
			AuditFields: map[string]string{
				"plugin": "ip_allowlist",
				"action": "deny",
				"match":  "allow_cidr_miss",
			},
		}, nil
	}
	return plugin.Result{
		Action: plugin.ActionAllow,
		AuditFields: map[string]string{
			"plugin": "ip_allowlist",
			"action": "allow",
		},
	}, nil
}

func parseClientIP(remoteAddr string) net.IP {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if remoteAddr == "" {
		return nil
	}
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		remoteAddr = host
	}
	return net.ParseIP(remoteAddr)
}

func compileCIDRs(values []string) ([]*net.IPNet, error) {
	var networks []*net.IPNet
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if !strings.Contains(value, "/") {
			ip := net.ParseIP(value)
			if ip == nil {
				_, _, err := net.ParseCIDR(value)
				return networks, err
			}
			if ip.To4() != nil {
				value += "/32"
			} else {
				value += "/128"
			}
		}
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			return networks, err
		}
		networks = append(networks, network)
	}
	return networks, nil
}

func ipInAnyCIDR(ip net.IP, networks []*net.IPNet) bool {
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
