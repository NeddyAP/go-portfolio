package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/NeddyAP/go-portfolio/internal/middleware"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")

	if !s.sameOriginAllowed(r) {
		writeJSONError(w, http.StatusForbidden, "Invalid origin")
		return
	}
	if !s.auth.IsConfigured() {
		writeJSONError(w, http.StatusServiceUnavailable, "Admin login is not configured on this server (ADMIN_PASSWORD_HASH is unset)")
		return
	}

	ip := middleware.RealIP(r, s.cfg.TrustProxy)
	allowed, retryAfter := s.auth.AllowLoginAttempt(ip)
	if !allowed {
		w.Header().Set("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())))
		writeJSONError(w, http.StatusTooManyRequests, "Too many failed login attempts. Please try again later.")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || req.Password == "" {
		writeJSONError(w, http.StatusBadRequest, "Email and password are required")
		return
	}

	if !s.auth.VerifyCredentials(req.Email, req.Password) {
		s.auth.RecordFailedLogin(ip)
		writeJSONError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	s.auth.ClearLoginFailures(ip)
	sess, err := s.auth.IssueSession(w, r, req.Email)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create session")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"email": sess.Email,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")

	if sess, ok := s.auth.GetSession(r); ok {
		if !s.auth.ValidateCSRF(r, sess) {
			writeJSONError(w, http.StatusForbidden, "Invalid CSRF token")
			return
		}
	}

	s.auth.ClearSession(w, r)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	if sess, ok := s.auth.GetSession(r); ok {
		writeJSON(w, http.StatusOK, map[string]any{
			"authenticated": true,
			"email":         sess.Email,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": false,
	})
}

// requireAdmin enforces authenticated session cookie and CSRF verification on mutating endpoints.
func (s *Server) requireAdmin(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")
		if !s.sameOriginAllowed(r) {
			writeJSONError(w, http.StatusForbidden, "Invalid origin")
			return
		}
		sess, ok := s.auth.GetSession(r)
		if !ok {
			writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		if !s.auth.ValidateCSRF(r, sess) {
			writeJSONError(w, http.StatusForbidden, "Invalid CSRF token")
			return
		}
		next(w, r)
	})
}
