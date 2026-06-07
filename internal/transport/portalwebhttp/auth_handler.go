package portalwebhttp

import (
	"net/http"
	"strings"

	portalapp "github.com/KnifeFly/token-gateway/internal/app/portal"
	"github.com/KnifeFly/token-gateway/pkg/apperr"
)

type sessionRequest struct {
	session   portalapp.Session
	principal portalapp.Principal
	requestID string
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	if h.portal == nil {
		writeError(w, requestID(r), apperr.ConfigUnavailable("portal web service is unavailable"))
		return
	}
	var request portalapp.APIKeyLoginRequest
	if !decodeJSON(w, requestID(r), r, &request) {
		return
	}
	response, err := h.portal.LoginWithAPIKey(r.Context(), request, r.UserAgent(), r.RemoteAddr)
	if err != nil {
		writeError(w, requestID(r), err)
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
	if err := h.portal.Logout(r.Context(), sr.session.ID); err != nil {
		writeError(w, sr.requestID, err)
		return
	}
	clearSessionCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request, sr sessionRequest) {
	response, err := h.portal.SessionResponse(r.Context(), sr.session.ID)
	if err != nil {
		writeError(w, sr.requestID, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) withSession(next func(http.ResponseWriter, *http.Request, sessionRequest)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := requestID(r)
		if h.portal == nil {
			writeError(w, requestID, apperr.ConfigUnavailable("portal web service is unavailable"))
			return
		}
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || strings.TrimSpace(cookie.Value) == "" {
			writeError(w, requestID, apperr.Unauthorized("portal session is required"))
			return
		}
		session, principal, err := h.portal.Session(r.Context(), cookie.Value)
		if err != nil {
			writeError(w, requestID, err)
			return
		}
		next(w, r, sessionRequest{session: session, principal: principal, requestID: requestID})
	}
}

func (h *Handler) requireCSRF(r *http.Request, sessionID string) error {
	return h.portal.ValidateCSRF(r.Context(), sessionID, r.Header.Get(csrfHeaderName))
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, session portalapp.SessionResponse) {
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
