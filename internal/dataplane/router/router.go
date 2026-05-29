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

// RoutePlanner resolves model permissions and ordered provider candidates.
type RoutePlanner struct {
	selector *PrioritySelector
}

func NewRoutePlanner(selector *PrioritySelector) *RoutePlanner {
	if selector == nil {
		selector = NewPrioritySelector(nil)
	}
	return &RoutePlanner{selector: selector}
}

func (p *RoutePlanner) Plan(_ context.Context, state *engine.RequestState) error {
	if state.Snapshot == nil {
		return apperr.ConfigUnavailable("runtime snapshot is unavailable")
	}
	model, ok := state.Snapshot.LookupModel(state.RequestedModel)
	if !ok || !model.Enabled {
		return apperr.NotFound("model not found")
	}
	if state.Principal == nil {
		return apperr.Unauthorized("authentication is required")
	}
	if !modelAllowed(state.Principal.AllowedModels, model.PublicModel) {
		return apperr.Forbidden("model is not allowed")
	}
	if !protocolMatchesModel(state.ProtocolMode, model.Protocol) {
		return apperr.InvalidArgument("model protocol does not match endpoint")
	}
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
	state.ResolvedModel = model
	state.RoutePlan = &engine.RoutePlan{
		PolicyID:   route.ID,
		Candidates: p.selector.Order(candidates),
	}
	return nil
}

func modelAllowed(allowed []string, model string) bool {
	for _, value := range allowed {
		if value == "*" || value == model {
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

func NewPrioritySelector(random *WeightedRandomSelector) *PrioritySelector {
	if random == nil {
		random = NewWeightedRandomSelector(rand.New(rand.NewSource(time.Now().UnixNano())))
	}
	return &PrioritySelector{random: random}
}

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

func NewWeightedRandomSelector(r *rand.Rand) *WeightedRandomSelector {
	if r == nil {
		r = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return &WeightedRandomSelector{r: r}
}

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
