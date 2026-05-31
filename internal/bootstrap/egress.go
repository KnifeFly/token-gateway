package bootstrap

import (
	"net/http"
	"time"

	"github.com/KnifeFly/token-gateway/pkg/egressguard"
)

func newEgressGuard(cfg EgressConfig) (*egressguard.Guard, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	return egressguard.New(egressguard.Config{
		AllowedHosts: cfg.AllowedHosts,
		AllowedCIDRs: cfg.AllowedCIDRs,
	})
}

func outboundHTTPClient(timeout time.Duration, guard *egressguard.Guard) *http.Client {
	if guard == nil && timeout <= 0 {
		return nil
	}
	client := &http.Client{Timeout: timeout}
	if guard != nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.DialContext = guard.DialContext
		client.Transport = transport
	}
	return client
}
