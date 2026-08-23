// Package admin serves the password-protected web UI and JSON API.
package admin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"time"

	"ooblivion/internal/auth"
	"ooblivion/internal/config"
	"ooblivion/internal/logx"
	"ooblivion/internal/scheduler"
	"ooblivion/internal/telegram"
	"ooblivion/internal/web"
)

const (
	cookieName = "oob_session"
	csrfName   = "oob_csrf"
)

type Server struct {
	db       *sql.DB
	cfg      *config.Config
	secret   []byte
	limiter  *auth.RateLimiter
	telegram *telegram.Sender
	sched    *scheduler.Scheduler
	tmpl     map[string]*template.Template
	logger   *logx.Logger
}

func New(db *sql.DB, cfg *config.Config, tg *telegram.Sender, sched *scheduler.Scheduler, logger *logx.Logger) (*Server, error) {
	base, err := template.New("base").Funcs(templateFuncs()).ParseFS(web.FS, "templates/base.html")
	if err != nil {
		return nil, err
	}
	tmpl := map[string]*template.Template{}
	for _, page := range []string{"login", "dashboard", "requests", "detail", "scopes", "notifications", "settings", "audit"} {
		clone, err := base.Clone()
		if err != nil {
			return nil, err
		}
		if _, err := clone.ParseFS(web.FS, "templates/"+page+".html"); err != nil {
			return nil, err
		}
		tmpl[page] = clone
	}
	return &Server{
		db:       db,
		cfg:      cfg,
		secret:   cfg.JWTSecret,
		limiter:  auth.NewRateLimiter(5, 10*time.Minute),
		telegram: tg,
		sched:    sched,
		tmpl:     tmpl,
		logger:   logger,
	}, nil
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	staticFS, err := fs.Sub(web.FS, "static")
	if err != nil {
		s.logger.Fatalf("static subfs: %v", err)
	}
	static := http.StripPrefix("/admin/static/", http.FileServer(http.FS(staticFS)))
	mux.Handle("GET /admin/static/", static)

	mux.HandleFunc("GET /admin/login", s.handleLoginPage)
	mux.HandleFunc("POST /admin/api/login", s.handleLogin)

	mux.HandleFunc("GET /admin/", s.auth(s.handleDashboard))
	mux.HandleFunc("GET /admin", s.auth(s.handleDashboard))
	mux.HandleFunc("POST /admin/api/logout", s.auth(s.csrf(s.handleLogout)))

	mux.HandleFunc("GET /admin/requests", s.auth(s.handleRequestsPage))
	mux.HandleFunc("GET /admin/requests/{id}", s.auth(s.handleDetailPage))
	mux.HandleFunc("GET /admin/scopes", s.auth(s.handleScopesPage))
	mux.HandleFunc("GET /admin/notifications", s.auth(s.handleNotificationsPage))
	mux.HandleFunc("GET /admin/settings", s.auth(s.handleSettingsPage))
	mux.HandleFunc("GET /admin/audit", s.auth(s.handleAuditPage))

	mux.HandleFunc("GET /admin/api/stats", s.auth(s.handleStats))
	mux.HandleFunc("GET /admin/api/requests", s.auth(s.handleListRequests))
	mux.HandleFunc("GET /admin/api/requests/{id}", s.auth(s.handleGetRequest))
	mux.HandleFunc("POST /admin/api/requests/{id}/save", s.auth(s.csrf(s.handleSaveRequest)))
	mux.HandleFunc("POST /admin/api/requests/{id}/unsave", s.auth(s.csrf(s.handleUnsaveRequest)))
	mux.HandleFunc("DELETE /admin/api/requests/{id}", s.auth(s.csrf(s.handleDeleteRequest)))
	mux.HandleFunc("POST /admin/api/requests/bulk_delete", s.auth(s.csrf(s.handleBulkDelete)))
	mux.HandleFunc("POST /admin/api/flush", s.auth(s.csrf(s.handleFlush)))

	mux.HandleFunc("GET /admin/api/scopes", s.auth(s.handleListScopes))
	mux.HandleFunc("POST /admin/api/scopes", s.auth(s.csrf(s.handleCreateScope)))
	mux.HandleFunc("PUT /admin/api/scopes/{id}", s.auth(s.csrf(s.handleUpdateScope)))
	mux.HandleFunc("DELETE /admin/api/scopes/{id}", s.auth(s.csrf(s.handleDeleteScope)))

	mux.HandleFunc("GET /admin/api/notifications", s.auth(s.handleListRules))
	mux.HandleFunc("POST /admin/api/notifications", s.auth(s.csrf(s.handleCreateRule)))
	mux.HandleFunc("PUT /admin/api/notifications/{id}", s.auth(s.csrf(s.handleUpdateRule)))
	mux.HandleFunc("DELETE /admin/api/notifications/{id}", s.auth(s.csrf(s.handleDeleteRule)))
	mux.HandleFunc("POST /admin/api/notifications/{id}/test", s.auth(s.csrf(s.handleTestRule)))

	mux.HandleFunc("GET /admin/api/settings", s.auth(s.handleGetSettings))
	mux.HandleFunc("PUT /admin/api/settings", s.auth(s.csrf(s.handlePutSettings)))
	mux.HandleFunc("POST /admin/api/password", s.auth(s.csrf(s.handleChangePassword)))

	mux.HandleFunc("GET /admin/api/audit", s.auth(s.handleAudit))

	return withSecurityHeaders(mux)
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set(
			"Content-Security-Policy",
			"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self'; img-src 'self' data:; base-uri 'none'; frame-ancestors 'none'",
		)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := s.currentClaims(r); err != nil {
			if isAPI(r) {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
				return
			}
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}
		next(w, r)
	}
}

func (s *Server) csrf(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !auth.CSRFValid(r) {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "invalid csrf token"})
			return
		}
		next(w, r)
	}
}

func (s *Server) tokenVersion() int {
	v, _ := strconv.Atoi(scheduler.ReadSetting(s.db, "token_version"))
	return v
}

func (s *Server) currentClaims(r *http.Request) (*auth.Claims, error) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return nil, errors.New("no session cookie")
	}
	claims, err := auth.ParseJWT(s.secret, cookie.Value)
	if err != nil {
		return nil, err
	}
	denied, err := auth.DenylistCheck(s.db, claims.ID)
	if err != nil {
		return nil, err
	}
	if denied || claims.Subject != "admin" || claims.Ver != s.tokenVersion() {
		return nil, errors.New("invalid session")
	}
	return claims, nil
}

func isAPI(r *http.Request) bool {
	return len(r.URL.Path) >= 10 && r.URL.Path[:10] == "/admin/api"
}

func (s *Server) render(w http.ResponseWriter, page string, data map[string]any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if data == nil {
		data = map[string]any{}
	}
	if _, ok := data["Authed"]; !ok {
		data["Authed"] = true
	}
	tmpl, ok := s.tmpl[page]
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		s.logger.Errorf("render %s: %v", page, err)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
