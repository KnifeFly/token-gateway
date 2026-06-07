package portalhttp

import (
	"encoding/json"
	"net/http"

	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

func writeResult(w http.ResponseWriter, requestID string, result any, err error) {
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, requestID string, err error) {
	status := http.StatusInternalServerError
	code := string(apperr.CodeInternal)
	message := "internal error"
	errType := "service_error"
	retryable := false
	if appErr, ok := apperr.As(err); ok {
		status = appErr.HTTPStatus
		code = string(appErr.Code)
		message = appErr.SafeMessage()
		retryable = appErr.Temporary
		errType = "invalid_request_error"
		if status >= 500 {
			errType = "service_error"
		}
		if status == http.StatusUnauthorized {
			errType = "authentication_error"
		}
		if status == http.StatusForbidden {
			errType = "permission_error"
		}
	}
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":       code,
			"message":    message,
			"type":       errType,
			"request_id": requestID,
			"retryable":  retryable,
		},
	})
}
