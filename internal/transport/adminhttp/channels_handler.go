package adminhttp

import (
	"net/http"
	"strings"

	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func (h *Handler) listChannels(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListChannels(r.Context(), sr.actor)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) channelRead(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	channelID, action, ok := parseChannelPath(r.URL.Path)
	if !ok {
		writeError(w, sr.requestID, apperr.NotFound("admin channel route not found"))
		return
	}
	if action == "" {
		response, err := h.admin.GetChannel(r.Context(), sr.actor, channelID)
		writeResult(w, sr.requestID, response, err)
		return
	}
	if action == "health-events" {
		response, err := h.admin.ListChannelHealthEvents(r.Context(), sr.actor, channelID)
		writeResult(w, sr.requestID, response, err)
		return
	}
	writeError(w, sr.requestID, apperr.NotFound("admin channel route not found"))
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

func (h *Handler) patchChannel(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	channelID, action, ok := parseChannelPath(r.URL.Path)
	if !ok || action != "" {
		writeError(w, sr.requestID, apperr.NotFound("admin channel route not found"))
		return
	}
	if !h.requireMutation(w, r, sr) {
		return
	}
	var request configadmin.ChannelConfig
	if !decodeJSON(w, sr.requestID, r, &request) {
		return
	}
	response, err := h.admin.PatchChannel(r.Context(), sr.actor, channelID, request, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) channelAction(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	channelID, action, ok := parseChannelPath(r.URL.Path)
	if !ok || action == "" {
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
	case "rotate-credential":
		var request struct {
			APIKey string `json:"api_key"`
		}
		if !decodeJSON(w, sr.requestID, r, &request) {
			return
		}
		response, err := h.admin.RotateChannelCredential(r.Context(), sr.actor, channelID, request.APIKey, h.mutationOptions(r, sr.requestID))
		writeResult(w, sr.requestID, response, err)
	case "test":
		response, err := h.admin.TestChannel(r.Context(), sr.actor, channelID, h.mutationOptions(r, sr.requestID))
		writeResult(w, sr.requestID, response, err)
	case "sync-preview":
		var request configadmin.ChannelModelSyncPreviewRequest
		if !decodeJSON(w, sr.requestID, r, &request) {
			return
		}
		request.ChannelID = channelID
		response, err := h.admin.PreviewChannelModelSync(r.Context(), sr.actor, request, h.mutationOptions(r, sr.requestID))
		writeResult(w, sr.requestID, response, err)
	case "sync-apply":
		var request configadmin.ChannelModelSyncPreviewRequest
		if !decodeJSON(w, sr.requestID, r, &request) {
			return
		}
		request.ChannelID = channelID
		response, err := h.admin.ApplyChannelModelSync(r.Context(), sr.actor, request, h.mutationOptions(r, sr.requestID))
		writeResult(w, sr.requestID, response, err)
	default:
		writeError(w, sr.requestID, apperr.NotFound("admin channel route not found"))
	}
}

func parseChannelPath(path string) (string, string, bool) {
	value := strings.Trim(strings.TrimPrefix(path, "/api/admin/v1/channels/"), "/")
	if value == "" {
		return "", "", false
	}
	parts := strings.Split(value, "/")
	if len(parts) == 1 && parts[0] != "" {
		return parts[0], "", true
	}
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return parts[0], parts[1], true
	}
	return "", "", false
}
