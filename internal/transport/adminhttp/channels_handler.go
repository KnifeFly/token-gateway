package adminhttp

import (
	"net/http"

	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func (h *Handler) listChannels(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListChannels(r.Context(), sr.actor)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) upsertChannel(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	var request configadmin.ChannelConfig
	if !decodeJSON(w, sr.requestID, r, &request) {
		return
	}
	response, err := h.admin.UpsertChannel(r.Context(), sr.actor, request, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) channelAction(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	channelID, action, ok := parseActionPath(r.URL.Path, "/api/admin/v1/channels/")
	if !ok {
		writeError(w, sr.requestID, apperr.NotFound("admin channel route not found"))
		return
	}
	if !h.requireMutation(w, r, sr) {
		return
	}
	switch action {
	case "enable":
		response, err := h.admin.SetChannelEnabled(r.Context(), sr.actor, channelID, true, h.mutationOptions(r, sr.requestID))
		writeResult(w, sr.requestID, response, err)
	case "disable":
		response, err := h.admin.SetChannelEnabled(r.Context(), sr.actor, channelID, false, h.mutationOptions(r, sr.requestID))
		writeResult(w, sr.requestID, response, err)
	case "test":
		writeError(w, sr.requestID, apperr.FeatureNotEnabled("channel test workflow is unavailable"))
	default:
		writeError(w, sr.requestID, apperr.NotFound("admin channel route not found"))
	}
}
