package adminhttp

import (
	"net/http"
	"strings"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func (h *Handler) listModels(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListModels(r.Context(), sr.actor)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) upsertModel(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	var request configadmin.ModelConfig
	if !decodeJSON(w, sr.requestID, r, &request) {
		return
	}
	response, err := h.admin.UpsertModel(r.Context(), sr.actor, request, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) modelRead(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	modelID, action := modelIDAndAction(r.URL.Path)
	if modelID == "" {
		writeError(w, sr.requestID, apperr.NotFound("admin model route not found"))
		return
	}
	switch action {
	case "":
		response, err := h.admin.GetModel(r.Context(), sr.actor, modelID)
		writeResult(w, sr.requestID, response, err)
	case "channels":
		response, err := h.admin.ListModelChannels(r.Context(), sr.actor, modelID)
		writeResult(w, sr.requestID, response, err)
	case "schema-preview":
		response, err := h.admin.GetModelSchemaPreview(r.Context(), sr.actor, modelID)
		writeResult(w, sr.requestID, response, err)
	default:
		writeError(w, sr.requestID, apperr.NotFound("admin model route not found"))
	}
}

func (h *Handler) patchModel(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	modelID, action := modelIDAndAction(r.URL.Path)
	if modelID == "" || action != "" {
		writeError(w, sr.requestID, apperr.NotFound("admin model route not found"))
		return
	}
	var request configadmin.ModelConfig
	if !decodeJSON(w, sr.requestID, r, &request) {
		return
	}
	response, err := h.admin.PatchModel(r.Context(), sr.actor, modelID, request, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) modelAction(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	modelID, action := modelIDAndAction(r.URL.Path)
	if modelID == "" {
		writeError(w, sr.requestID, apperr.NotFound("admin model route not found"))
		return
	}
	switch action {
	case "disable":
		response, err := h.admin.SetModelEnabled(r.Context(), sr.actor, modelID, false, h.mutationOptions(r, sr.requestID))
		writeResult(w, sr.requestID, response, err)
	case "deprecate":
		response, err := h.admin.DeprecateModel(r.Context(), sr.actor, modelID, h.mutationOptions(r, sr.requestID))
		writeResult(w, sr.requestID, response, err)
	default:
		writeError(w, sr.requestID, apperr.NotFound("admin model route not found"))
	}
}

func (h *Handler) previewModelCatalogSync(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	var request adminapp.ModelCatalogSyncPreviewRequest
	if !decodeJSON(w, sr.requestID, r, &request) {
		return
	}
	response, err := h.admin.PreviewModelCatalogSync(r.Context(), sr.actor, request, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func modelIDAndAction(path string) (string, string) {
	value := strings.Trim(strings.TrimPrefix(path, "/api/admin/v1/models/"), "/")
	if value == "" {
		return "", ""
	}
	parts := strings.Split(value, "/")
	if len(parts) == 1 {
		return parts[0], ""
	}
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return parts[0], strings.Join(parts[1:], "/")
}
