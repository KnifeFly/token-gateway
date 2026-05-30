package router

import (
	"context"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/KnifeFly/token-gateway/internal/dataplane/engine"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

// router.go resolves public models into ordered provider candidates using snapshot and hot signals.

// RoutePlanner resolves model permissions and ordered provider candidates.
type RoutePlanner struct {
	selector *PrioritySelector
	registry *StrategyRegistry
	signals  SignalProvider
	disable  DisableChecker
}

// DisableChecker checks emergency provider/channel disables.
type DisableChecker interface {
	IsProviderDisabled(ctx context.Context, providerType string) (bool, error)
	IsChannelDisabled(ctx context.Context, channelID string) (bool, error)
}

// NewRoutePlanner returns a route planner with default priority strategies.
func NewRoutePlanner(selector *PrioritySelector, disable ...DisableChecker) *RoutePlanner {
	if selector == nil {
		selector = NewPrioritySelector(nil)
	}
	p := &RoutePlanner{selector: selector, registry: NewStrategyRegistry(selector)}
	if len(disable) > 0 {
		p.disable = disable[0]
	}
	return p
}

// WithSignals configures the optional hot-signal provider.
func (p *RoutePlanner) WithSignals(provider SignalProvider) *RoutePlanner {
	if p == nil {
		return p
	}
	p.signals = provider
	return p
}

// Plan validates model access and writes an ordered provider route plan.
func (p *RoutePlanner) Plan(ctx context.Context, state *engine.RequestState) error {
	if state.Snapshot == nil {
		return apperr.ConfigUnavailable("runtime snapshot is unavailable")
	}

	// Step 1: resolve and authorize the requested public model.
	model, ok := state.Snapshot.LookupModel(state.RequestedModel)
	if !ok || !model.Enabled {
		return apperr.NotFound("model not found")
	}
	if state.Principal == nil {
		return apperr.Unauthorized("authentication is required")
	}
	if !modelAllowedForView(state.Principal.AllowedModels, state.RequestedModel, model) {
		return apperr.Forbidden("model is not allowed")
	}
	if !protocolMatchesModel(state.ProtocolMode, model.Protocol) {
		return apperr.InvalidArgument("model protocol does not match endpoint")
	}

	// Step 2: expand the route policy into provider candidates from the snapshot.
	route, ok := state.Snapshot.LookupRoute(model.PublicModel)
	if !ok || len(route.Candidates) == 0 {
		return apperr.ServiceUnavailable("no route is available", apperr.WithTemporary())
	}
	candidates := make([]engine.ProviderCandidate, 0, len(route.Candidates))
	for _, candidate := range route.Candidates {
		channel, ok := state.Snapshot.LookupChannel(candidate.ChannelID)
		if !ok || !channel.Enabled {
			continue
		}
		upstreamModel := channel.Models[model.PublicModel]
		if upstreamModel == "" {
			continue
		}
		candidates = append(candidates, engine.ProviderCandidate{
			ChannelID:     channel.ID,
			ProviderType:  channel.ProviderType,
			PublicModel:   model.PublicModel,
			UpstreamModel: upstreamModel,
			Priority:      candidate.Priority,
			Weight:        candidate.Weight,
			Timeout:       channel.Timeout,
		})
	}
	if len(candidates) == 0 {
		return apperr.ServiceUnavailable("no provider channel is available", apperr.WithTemporary())
	}

	// Step 3: merge hot signals and persist the ordered plan on request state.
	signals, err := p.routeSignals(ctx, state, candidates)
	if err != nil {
		return err
	}
	p.storeCircuitStates(state, candidates, signals)
	ordered := p.registry.Order(route.Strategy, candidates, signals)
	if len(ordered) == 0 {
		return noRouteAvailable()
	}
	state.ResolvedModel = model
	if price, ok := state.Snapshot.LookupPrice(model.PublicModel); ok {
		state.PriceRule = price
	}
	if limit, ok := state.Snapshot.LookupLimit(model.PublicModel); ok {
		state.LimitRule = limit
	}
	state.RoutePlan = &engine.RoutePlan{
		PolicyID:   route.ID,
		Candidates: ordered,
	}
	return nil
}

func (p *RoutePlanner) routeSignals(ctx context.Context, state *engine.RequestState, candidates []engine.ProviderCandidate) (RouteSignals, error) {
	signals := RouteSignals{Candidates: map[string]CandidateSignal{}}
	if p.signals != nil {
		loaded, err := p.signals.Signals(ctx, state, candidates)
		if err != nil {
			return RouteSignals{}, err
		}
		signals = loaded
		if signals.Candidates == nil {
			signals.Candidates = map[string]CandidateSignal{}
		}
	}
	for _, candidate := range candidates {
		signal, ok := signals.Candidates[candidate.ChannelID]
		if !ok {
			signal = defaultCandidateSignal()
		}
		disabled, err := p.isDisabled(ctx, candidate.ProviderType, candidate.ChannelID)
		if err != nil {
			return RouteSignals{}, err
		}
		signal.Disabled = signal.Disabled || disabled
		if signal.HealthWeight == 0 {
			signal.HealthWeight = 1
		}
		if signal.CircuitState == "" {
			signal.CircuitState = CircuitClosed
		}
		signals.Candidates[candidate.ChannelID] = signal
	}
	return signals, nil
}

func (p *RoutePlanner) storeCircuitStates(state *engine.RequestState, candidates []engine.ProviderCandidate, signals RouteSignals) {
	if state == nil {
		return
	}
	if state.Internal == nil {
		state.Internal = map[string]any{}
	}
	states := make(map[string]string, len(candidates))
	for _, candidate := range candidates {
		circuitState := signals.signal(candidate).CircuitState
		if circuitState == "" {
			circuitState = CircuitClosed
		}
		states[candidate.ChannelID] = circuitState
	}
	state.Internal["route.circuit_states"] = states
}

func defaultCandidateSignal() CandidateSignal {
	return CandidateSignal{Healthy: true, HealthWeight: 1, ModelCompatible: true, CircuitState: CircuitClosed}
}

func (p *RoutePlanner) isDisabled(ctx context.Context, providerType, channelID string) (bool, error) {
	if p.disable == nil {
		return false, nil
	}
	providerDisabled, err := p.disable.IsProviderDisabled(ctx, providerType)
	if err != nil || providerDisabled {
		return providerDisabled, err
	}
	return p.disable.IsChannelDisabled(ctx, channelID)
}

func modelAllowed(allowed []string, model string) bool {
	for _, value := range allowed {
		if value == "*" || value == model {
			return true
		}
	}
	return false
}

func modelAllowedForView(allowed []string, requested string, model engine.ModelView) bool {
	if modelAllowed(allowed, model.PublicModel) || modelAllowed(allowed, requested) {
		return true
	}
	for _, alias := range model.Aliases {
		if modelAllowed(allowed, alias) {
			return true
		}
	}
	return false
}

func protocolMatchesModel(requestMode engine.ProtocolMode, modelMode engine.ProtocolMode) bool {
	if requestMode == "" || modelMode == "" {
		return true
	}
	return requestMode == modelMode
}

// PrioritySelector orders lower priority numbers first and shuffles equal
// priority candidates using weighted random.
type PrioritySelector struct {
	random *WeightedRandomSelector
}

// NewPrioritySelector returns a priority selector with weighted tie-breaking.
func NewPrioritySelector(random *WeightedRandomSelector) *PrioritySelector {
	if random == nil {
		random = NewWeightedRandomSelector(rand.New(rand.NewSource(time.Now().UnixNano())))
	}
	return &PrioritySelector{random: random}
}

// Order returns candidates grouped by priority and shuffled by weight within a group.
func (s *PrioritySelector) Order(candidates []engine.ProviderCandidate) []engine.ProviderCandidate {
	ordered := append([]engine.ProviderCandidate(nil), candidates...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Priority < ordered[j].Priority
	})
	var out []engine.ProviderCandidate
	for len(ordered) > 0 {
		priority := ordered[0].Priority
		end := 0
		for end < len(ordered) && ordered[end].Priority == priority {
			end++
		}
		group := append([]engine.ProviderCandidate(nil), ordered[:end]...)
		for len(group) > 0 {
			selected := s.random.Pick(group)
			out = append(out, group[selected])
			group = append(group[:selected], group[selected+1:]...)
		}
		ordered = ordered[end:]
	}
	return out
}

// WeightedRandomSelector picks candidates proportionally by positive weight.
type WeightedRandomSelector struct {
	mu sync.Mutex
	r  *rand.Rand
}

// NewWeightedRandomSelector returns a selector using r or a time-seeded random source.
func NewWeightedRandomSelector(r *rand.Rand) *WeightedRandomSelector {
	if r == nil {
		r = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return &WeightedRandomSelector{r: r}
}

// Pick returns the index selected by positive candidate weights.
func (s *WeightedRandomSelector) Pick(candidates []engine.ProviderCandidate) int {
	if len(candidates) <= 1 {
		return 0
	}
	total := 0
	for _, candidate := range candidates {
		if candidate.Weight > 0 {
			total += candidate.Weight
		}
	}
	if total <= 0 {
		return 0
	}
	s.mu.Lock()
	n := s.r.Intn(total)
	s.mu.Unlock()
	for i, candidate := range candidates {
		if candidate.Weight <= 0 {
			continue
		}
		if n < candidate.Weight {
			return i
		}
		n -= candidate.Weight
	}
	return len(candidates) - 1
}
