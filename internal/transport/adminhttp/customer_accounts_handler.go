package adminhttp

import (
	"net/http"
	"strings"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func (h *Handler) listCustomerAccounts(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.ListCustomerAccounts(r.Context(), sr.actor, adminapp.CustomerAccountFilter{
		TenantID:  r.URL.Query().Get("tenant_id"),
		ProjectID: r.URL.Query().Get("project_id"),
		Status:    r.URL.Query().Get("status"),
		Keyword:   r.URL.Query().Get("keyword"),
	})
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) createCustomerAccount(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if !h.requireMutation(w, r, sr) {
		return
	}
	var request adminapp.CustomerAccountCreateRequest
	if !decodeJSON(w, sr.requestID, r, &request) {
		return
	}
	response, err := h.admin.CreateCustomerAccount(r.Context(), sr.actor, request, h.mutationOptions(r, sr.requestID))
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) customerAccountRead(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	accountID, action := customerAccountIDAndAction(r.URL.Path)
	if accountID == "" || action != "" {
		writeError(w, sr.requestID, apperr.NotFound("admin customer account route not found"))
		return
	}
	response, err := h.admin.GetCustomerAccount(r.Context(), sr.actor, accountID)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) customerAccountAction(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	accountID, action := customerAccountIDAndAction(r.URL.Path)
	if accountID == "" || action == "" {
		writeError(w, sr.requestID, apperr.NotFound("admin customer account route not found"))
		return
	}
	if !h.requireMutation(w, r, sr) {
		return
	}
	switch action {
	case "enable":
		response, err := h.admin.SetCustomerAccountEnabled(r.Context(), sr.actor, accountID, true, h.mutationOptions(r, sr.requestID))
		writeResult(w, sr.requestID, response, err)
	case "disable":
		response, err := h.admin.SetCustomerAccountEnabled(r.Context(), sr.actor, accountID, false, h.mutationOptions(r, sr.requestID))
		writeResult(w, sr.requestID, response, err)
	case "manual-adjustment":
		var request adminapp.CustomerCreditAdjustmentRequest
		if !decodeJSON(w, sr.requestID, r, &request) {
			return
		}
		response, err := h.admin.AdjustCustomerCredits(r.Context(), sr.actor, accountID, request, h.mutationOptions(r, sr.requestID))
		writeResult(w, sr.requestID, response, err)
	case "reset-session":
		var request struct {
			APIKeyID string `json:"api_key_id"`
		}
		if r.Body != http.NoBody && r.ContentLength != 0 && !decodeJSON(w, sr.requestID, r, &request) {
			return
		}
		response, err := h.admin.ResetCustomerPortalSessions(r.Context(), sr.actor, accountID, request.APIKeyID, h.mutationOptions(r, sr.requestID))
		writeResult(w, sr.requestID, response, err)
	default:
		writeError(w, sr.requestID, apperr.NotFound("admin customer account route not found"))
	}
}

func customerAccountIDAndAction(path string) (string, string) {
	value := strings.Trim(strings.TrimPrefix(path, "/api/admin/v1/customer-accounts/"), "/")
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
