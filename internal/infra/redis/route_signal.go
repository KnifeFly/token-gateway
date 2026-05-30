package redis

import (
	"context"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// RouteSignal is one hot routing signal snapshot for a provider channel.
type RouteSignal struct {
	Healthy          *bool
	HealthWeight     float64
	Latency          time.Duration
	CostMicros       int64
	RemainingQuota   int64
	Disabled         *bool
	ModelCompatible  *bool
	SuccessRate      float64
	ErrorRate        float64
	RateLimited      int64
	ServerErrors     int64
	Timeouts         int64
	StreamInterrupts int64
	CircuitState     string
}

// RouteSignalStore reads and writes hot route signals in Redis.
type RouteSignalStore struct {
	client *goredis.Client
	prefix string
}

// NewRouteSignalStore returns a Redis-backed route signal store.
func NewRouteSignalStore(client *goredis.Client, prefix string) *RouteSignalStore {
	if prefix == "" {
		prefix = "token-gateway"
	}
	return &RouteSignalStore{client: client, prefix: prefix}
}

// GetRouteSignals reads route signals for channelIDs from Redis.
func (s *RouteSignalStore) GetRouteSignals(ctx context.Context, channelIDs []string) (map[string]RouteSignal, error) {
	out := make(map[string]RouteSignal, len(channelIDs))
	if s == nil || s.client == nil || len(channelIDs) == 0 {
		return out, nil
	}
	pipe := s.client.Pipeline()
	cmds := make(map[string]*goredis.MapStringStringCmd, len(channelIDs))
	for _, channelID := range channelIDs {
		channelID = strings.TrimSpace(channelID)
		if channelID == "" {
			continue
		}
		cmds[channelID] = pipe.HGetAll(ctx, s.key(channelID))
	}
	if _, err := pipe.Exec(ctx); err != nil && err != goredis.Nil {
		return nil, err
	}
	for channelID, cmd := range cmds {
		values, err := cmd.Result()
		if err != nil {
			return nil, err
		}
		if len(values) == 0 {
			continue
		}
		out[channelID] = parseRouteSignal(values)
	}
	return out, nil
}

// SetRouteSignal writes one route signal with a TTL for tests and control-plane publishers.
func (s *RouteSignalStore) SetRouteSignal(ctx context.Context, channelID string, signal RouteSignal, ttl time.Duration) error {
	if s == nil || s.client == nil || strings.TrimSpace(channelID) == "" {
		return nil
	}
	values := map[string]any{}
	if signal.Healthy != nil {
		values["healthy"] = boolString(*signal.Healthy)
	}
	if signal.HealthWeight > 0 {
		values["health_weight"] = strconv.FormatFloat(signal.HealthWeight, 'f', -1, 64)
	}
	if signal.Latency > 0 {
		values["latency_ms"] = signal.Latency.Milliseconds()
	}
	if signal.CostMicros > 0 {
		values["cost_micros"] = signal.CostMicros
	}
	if signal.RemainingQuota > 0 {
		values["remaining_quota"] = signal.RemainingQuota
	}
	if signal.Disabled != nil {
		values["disabled"] = boolString(*signal.Disabled)
	}
	if signal.ModelCompatible != nil {
		values["model_compatible"] = boolString(*signal.ModelCompatible)
	}
	if signal.SuccessRate > 0 {
		values["success_rate"] = strconv.FormatFloat(signal.SuccessRate, 'f', -1, 64)
	}
	if signal.ErrorRate > 0 {
		values["error_rate"] = strconv.FormatFloat(signal.ErrorRate, 'f', -1, 64)
	}
	if signal.RateLimited > 0 {
		values["rate_limited"] = signal.RateLimited
	}
	if signal.ServerErrors > 0 {
		values["server_errors"] = signal.ServerErrors
	}
	if signal.Timeouts > 0 {
		values["timeouts"] = signal.Timeouts
	}
	if signal.StreamInterrupts > 0 {
		values["stream_interrupts"] = signal.StreamInterrupts
	}
	if signal.CircuitState != "" {
		values["circuit_state"] = signal.CircuitState
	}
	if len(values) == 0 {
		return nil
	}
	key := s.key(channelID)
	if err := s.client.HSet(ctx, key, values).Err(); err != nil {
		return err
	}
	if ttl > 0 {
		return s.client.Expire(ctx, key, ttl).Err()
	}
	return nil
}

func (s *RouteSignalStore) key(channelID string) string {
	return s.prefix + ":route_signal:channel:" + strings.TrimSpace(channelID)
}

func parseRouteSignal(values map[string]string) RouteSignal {
	return RouteSignal{
		Healthy:          optionalBool(values["healthy"]),
		HealthWeight:     floatValue(values["health_weight"]),
		Latency:          time.Duration(int64Value(values["latency_ms"])) * time.Millisecond,
		CostMicros:       int64Value(values["cost_micros"]),
		RemainingQuota:   int64Value(values["remaining_quota"]),
		Disabled:         optionalBool(values["disabled"]),
		ModelCompatible:  optionalBool(values["model_compatible"]),
		SuccessRate:      floatValue(values["success_rate"]),
		ErrorRate:        floatValue(values["error_rate"]),
		RateLimited:      int64Value(values["rate_limited"]),
		ServerErrors:     int64Value(values["server_errors"]),
		Timeouts:         int64Value(values["timeouts"]),
		StreamInterrupts: int64Value(values["stream_interrupts"]),
		CircuitState:     strings.TrimSpace(values["circuit_state"]),
	}
}

func optionalBool(value string) *bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes":
		v := true
		return &v
	case "0", "false", "no":
		v := false
		return &v
	default:
		return nil
	}
}

func boolString(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func int64Value(value string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return n
}

func floatValue(value string) float64 {
	n, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return n
}
