package adminhttp

import (
	"net/http"
	"strings"

	"github.com/KnifeFly/token-gateway/internal/controlplane/configadmin"
)

func (h *Handler) listTenants(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListTenants(r.Context(), sr.actor)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) getTenant(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	tenantID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/v1/tenants/"), "/")
	response, err := h.admin.GetTenant(r.Context(), sr.actor, tenantID)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) upsertTenant(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	var request configadmin.Tenant
	if !decodeJSON(w, sr.requestID, r, &request) {
		return
	}
	response, err := h.admin.UpsertTenant(r.Context(), sr.actor, request, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}
