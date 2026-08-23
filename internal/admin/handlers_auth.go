package admin

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ooblivion/internal/auth"
	"ooblivion/internal/scheduler"
)

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, "login", map[string]any{"Title": "Login", "Authed": false})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := auth.ClientIP(r)
	if !s.limiter.Allow(ip) {
		writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "too many attempts"})
		return
	}

	var body struct {
		Password string `json:"password"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid body"})
		return
	}

	var hash string
	err := s.db.QueryRow("SELECT password_hash FROM auth WHERE id = 1").Scan(&hash)
	if err == sql.ErrNoRows || (err == nil && !verifyHash(hash, body.Password)) {
		s.logAudit("login_failed", "wrong password", ip)
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid credentials"})
		return
	}
	if err != nil {
		s.logger.Errorf("login lookup: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
		return
	}

	token, _, err := auth.IssueJWT(s.secret, time.Duration(s.cfg.SessionTTL)*time.Hour, s.tokenVersion())
	if err != nil {
		s.logger.Errorf("jwt issue: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
		return
	}

	csrf, err := auth.NewCSRFToken()
	if err != nil {
		s.logger.Errorf("csrf token: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
		return
	}

	setCookie(w, r, cookieName, token, s.cfg.SessionTTL*3600, true)
	setCookie(w, r, csrfName, csrf, s.cfg.SessionTTL*3600, false)
	s.logAudit("login", "admin logged in", ip)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(cookieName)
	if err == nil {
		if claims, perr := auth.ParseJWT(s.secret, cookie.Value); perr == nil && claims.ExpiresAt != nil {
			_ = auth.DenylistAdd(s.db, claims.ID, claims.ExpiresAt.Time)
		}
	}
	clearCookie(w, r, cookieName)
	clearCookie(w, r, csrfName)
	s.logAudit("logout", "admin logged out", auth.ClientIP(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid body"})
		return
	}
	if len(body.New) < 16 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "new password must be at least 16 chars"})
		return
	}

	var hash string
	if err := s.db.QueryRow("SELECT password_hash FROM auth WHERE id = 1").Scan(&hash); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
		return
	}
	if !verifyHash(hash, body.Current) {
		s.logAudit("password_change_failed", "wrong current password", auth.ClientIP(r))
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "current password incorrect"})
		return
	}

	newHash, err := auth.HashPassword(body.New)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.Exec(
		"UPDATE auth SET password_hash = ?, updated_at = ? WHERE id = 1", newHash, now); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
		return
	}

	next := s.tokenVersion() + 1
	if err := scheduler.WriteSetting(s.db, "token_version", strconv.Itoa(next)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
		return
	}

	if cookie, err := r.Cookie(cookieName); err == nil {
		if claims, perr := auth.ParseJWT(s.secret, cookie.Value); perr == nil && claims.ExpiresAt != nil {
			_ = auth.DenylistAdd(s.db, claims.ID, claims.ExpiresAt.Time)
		}
	}
	clearCookie(w, r, cookieName)
	s.logAudit("password_change", "admin changed password", auth.ClientIP(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) logAudit(action, detail, ip string) {
	_, err := s.db.Exec(
		"INSERT INTO audit_log (actor, action, detail, ip, created_at) VALUES ('admin', ?, ?, ?, ?)",
		action, detail, ip, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		s.logger.Errorf("audit insert: %v", err)
	}
}

func verifyHash(hash, password string) bool {
	ok, err := auth.VerifyPassword(hash, password)
	return err == nil && ok
}

func setCookie(w http.ResponseWriter, r *http.Request, name, value string, maxAge int, httpOnly bool) {
	secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	// #nosec G124 -- Secure is set deliberately when TLS/proxy-https is present.
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/admin",
		MaxAge:   maxAge,
		HttpOnly: httpOnly,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func clearCookie(w http.ResponseWriter, r *http.Request, name string) {
	secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	// #nosec G124 -- Secure is set deliberately when TLS/proxy-https is present.
	http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/admin", MaxAge: -1, Secure: secure, SameSite: http.SameSiteStrictMode})
}
