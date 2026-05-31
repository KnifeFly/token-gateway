package egressguard

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

// Resolver resolves hostnames for outbound safety checks.
type Resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

// Config controls outbound URL validation.
type Config struct {
	AllowedHosts []string
	AllowedCIDRs []string
	Resolver     Resolver
}

// Guard validates outbound URLs and dial targets before the gateway connects.
type Guard struct {
	resolver     Resolver
	allowedHosts map[string]struct{}
	allowedCIDRs []netip.Prefix
}

// New returns an outbound URL guard.
func New(cfg Config) (*Guard, error) {
	guard := &Guard{
		resolver:     cfg.Resolver,
		allowedHosts: make(map[string]struct{}, len(cfg.AllowedHosts)),
	}
	if guard.resolver == nil {
		guard.resolver = net.DefaultResolver
	}
	for _, host := range cfg.AllowedHosts {
		host = normalizeHost(host)
		if host != "" {
			guard.allowedHosts[host] = struct{}{}
		}
	}
	for _, cidr := range cfg.AllowedCIDRs {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(cidr))
		if err != nil {
			return nil, fmt.Errorf("invalid egress allow cidr %q: %w", cidr, err)
		}
		guard.allowedCIDRs = append(guard.allowedCIDRs, prefix)
	}
	return guard, nil
}

// ValidateURL rejects outbound URLs that target unsafe schemes, hosts, or IP ranges.
func (g *Guard) ValidateURL(ctx context.Context, raw string) error {
	if g == nil {
		return nil
	}
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("egress url is invalid")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		return fmt.Errorf("egress scheme %q is not allowed", parsed.Scheme)
	}
	return g.validateHost(ctx, parsed.Hostname())
}

// DialContext validates the requested host and the connected remote address.
func (g *Guard) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if err := g.validateHost(ctx, host); err != nil {
		return nil, err
	}
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	if tcp, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		addr, ok := netip.AddrFromSlice(tcp.IP)
		if ok {
			if err := g.validateAddr(addr.Unmap()); err != nil {
				_ = conn.Close()
				return nil, err
			}
		}
	}
	return conn, nil
}

func (g *Guard) validateHost(ctx context.Context, host string) error {
	host = normalizeHost(host)
	if host == "" {
		return fmt.Errorf("egress host is required")
	}
	if isMetadataHost(host) {
		return fmt.Errorf("egress host %q is not allowed", host)
	}
	if len(g.allowedHosts) > 0 {
		if _, ok := g.allowedHosts[host]; !ok {
			return fmt.Errorf("egress host %q is not allowlisted", host)
		}
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return g.validateAddr(addr.Unmap())
	}
	addrs, err := g.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("egress host %q could not be resolved: %w", host, err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("egress host %q resolved no addresses", host)
	}
	for _, resolved := range addrs {
		addr, ok := netip.AddrFromSlice(resolved.IP)
		if !ok {
			return fmt.Errorf("egress host %q resolved invalid address", host)
		}
		if err := g.validateAddr(addr.Unmap()); err != nil {
			return fmt.Errorf("egress host %q resolved to unsafe address %s: %w", host, addr.Unmap(), err)
		}
	}
	return nil
}

func (g *Guard) validateAddr(addr netip.Addr) error {
	if !addr.IsValid() {
		return fmt.Errorf("egress address is invalid")
	}
	for _, prefix := range g.allowedCIDRs {
		if prefix.Contains(addr) {
			return nil
		}
	}
	for _, prefix := range blockedPrefixes {
		if prefix.Contains(addr) {
			return fmt.Errorf("egress address %s is not allowed", addr)
		}
	}
	if addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified() {
		return fmt.Errorf("egress address %s is not allowed", addr)
	}
	return nil
}

func normalizeHost(host string) string {
	host = strings.TrimSpace(strings.TrimSuffix(host, "."))
	host = strings.Trim(host, "[]")
	return strings.ToLower(host)
}

func isMetadataHost(host string) bool {
	switch host {
	case "metadata", "metadata.google.internal", "169.254.169.254", "100.100.100.200":
		return true
	default:
		return false
	}
}

var blockedPrefixes = mustPrefixes(
	"0.0.0.0/8",
	"10.0.0.0/8",
	"100.64.0.0/10",
	"127.0.0.0/8",
	"169.254.0.0/16",
	"172.16.0.0/12",
	"192.0.0.0/24",
	"192.0.2.0/24",
	"192.168.0.0/16",
	"198.18.0.0/15",
	"198.51.100.0/24",
	"203.0.113.0/24",
	"224.0.0.0/4",
	"240.0.0.0/4",
	"::/128",
	"::1/128",
	"100::/64",
	"2001:db8::/32",
	"fc00::/7",
	"fe80::/10",
	"ff00::/8",
)

func mustPrefixes(values ...string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			panic(err)
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes
}
