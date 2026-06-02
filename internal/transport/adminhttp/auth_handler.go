package adminhttp

import (
	"net/http"
	"strings"

	adminapp "github.com/KnifeFly/token-gateway/internal/app/admin"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

type sessionRequest struct {
	session   adminapp.Session
	actor     adminapp.Actor
	requestID string
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	requestID := requestID(r)
	if h.admin == nil {
		writeError(w, requestID, apperr.ConfigUnavailable("admin web service is unavailable"))
		return
	}
	var request adminapp.LoginRequest
	if !decodeJSON(w, requestID, r, &request) {
		return
	}
	response, err := h.admin.Login(r.Context(), request, r.UserAgent(), r.RemoteAddr)
	if err != nil {
		writeError(w, requestID, err)
		return
	}
	setSessionCookie(w, r, response.Session)
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	if err := h.requireCSRF(r, sr.session.ID); err != nil {
		writeError(w, sr.requestID, err)
		return
	}
	if err := h.admin.Logout(r.Context(), sr.session.ID); err != nil {
		writeError(w, sr.requestID, err)
		return
	}
	clearSessionCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.admin.SessionResponse(r.Context(), sr.session.ID)
	writeResult(w, sr.requestID, response, err)
}

func (h *Handler) withSession(next func(http.ResponseWriter, *http.Request, sessionRequest)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := requestID(r)
		if h.admin == nil {
			writeError(w, requestID, apperr.ConfigUnavailable("admin web service is unavailable"))
			return
		}
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || strings.TrimSpace(cookie.Value) == "" {
			writeError(w, requestID, apperr.Unauthorized("admin session is required"))
			return
		}
		session, actor, err := h.admin.Session(r.Context(), cookie.Value)
		if err != nil {
			writeError(w, requestID, err)
			return
		}
		next(w, r, sessionRequest{session: session, actor: actor, requestID: requestID})
	}
}

func (h *Handler) requireMutation(w http.ResponseWriter, r *http.Request, sr sessionRequest) bool {
	if err := h.requireCSRF(r, sr.session.ID); err != nil {
		writeError(w, sr.requestID, err)
		return false
	}
	return true
}

func (h *Handler) requireCSRF(r *http.Request, sessionID string) error {
	return h.admin.ValidateCSRF(r.Context(), sessionID, r.Header.Get(csrfHeaderName))
}

func (h *Handler) mutationOptions(r *http.Request, requestID string) adminapp.MutationOptions {
	return adminapp.MutationOptions{
		RequestID:      requestID,
		IdempotencyKey: strings.TrimSpace(r.Header.Get("Idempotency-Key")),
		Reason:         strings.TrimSpace(r.Header.Get(reasonHeaderName)),
		RemoteAddr:     r.RemoteAddr,
		UserAgent:      r.UserAgent(),
	}
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, session adminapp.SessionResponse) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    session.SessionID,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secureCookie(r),
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secureCookie(r),
	})
}

func secureCookie(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}
