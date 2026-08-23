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
	"reflect"
	"strconv"
	"strings"
	"time"

	"ooblivion/internal/auth"
	"ooblivion/internal/config"
	"ooblivion/internal/logx"
	"ooblivion/internal/scheduler"
	"ooblivion/internal/telegram"
	"ooblivion/internal/version"
	"ooblivion/internal/web"
)

const (
	cookieName = "oob_session"
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
	mux.HandleFunc("POST /admin/api/login", s.formAware(s.handleLogin))

	mux.HandleFunc("GET /admin/", s.auth(s.handleDashboard))
	mux.HandleFunc("GET /admin", s.auth(s.handleDashboard))
	mux.HandleFunc("POST /admin/api/logout", s.formAware(s.auth(s.handleLogout)))

	mux.HandleFunc("GET /admin/requests", s.auth(s.handleRequestsPage))
	mux.HandleFunc("GET /admin/requests/{id}", s.auth(s.handleDetailPage))
	mux.HandleFunc("GET /admin/scopes", s.auth(s.handleScopesPage))
	mux.HandleFunc("GET /admin/notifications", s.auth(s.handleNotificationsPage))
	mux.HandleFunc("GET /admin/settings", s.auth(s.handleSettingsPage))
	mux.HandleFunc("GET /admin/audit", s.auth(s.handleAuditPage))

	mux.HandleFunc("GET /admin/api/stats", s.auth(s.handleStats))
	mux.HandleFunc("GET /admin/api/requests", s.auth(s.handleListRequests))
	mux.HandleFunc("GET /admin/api/requests/{id}", s.auth(s.handleGetRequest))
	mux.HandleFunc("POST /admin/api/requests/{id}/save", s.formAware(s.auth(s.handleSaveRequest)))
	mux.HandleFunc("POST /admin/api/requests/{id}/unsave", s.formAware(s.auth(s.handleUnsaveRequest)))
	mux.HandleFunc("DELETE /admin/api/requests/{id}", s.formAware(s.auth(s.handleDeleteRequest)))
	mux.HandleFunc("POST /admin/api/requests/bulk_delete", s.formAware(s.auth(s.handleBulkDelete)))
	mux.HandleFunc("POST /admin/api/flush", s.formAware(s.auth(s.handleFlush)))

	mux.HandleFunc("GET /admin/api/scopes", s.auth(s.handleListScopes))
	mux.HandleFunc("POST /admin/api/scopes", s.formAware(s.auth(s.handleCreateScope)))
	mux.HandleFunc("PUT /admin/api/scopes/{id}", s.formAware(s.auth(s.handleUpdateScope)))
	mux.HandleFunc("DELETE /admin/api/scopes/{id}", s.formAware(s.auth(s.handleDeleteScope)))

	mux.HandleFunc("GET /admin/api/notifications", s.auth(s.handleListRules))
	mux.HandleFunc("POST /admin/api/notifications", s.formAware(s.auth(s.handleCreateRule)))
	mux.HandleFunc("PUT /admin/api/notifications/{id}", s.formAware(s.auth(s.handleUpdateRule)))
	mux.HandleFunc("DELETE /admin/api/notifications/{id}", s.formAware(s.auth(s.handleDeleteRule)))
	mux.HandleFunc("POST /admin/api/notifications/{id}/test", s.formAware(s.auth(s.handleTestRule)))

	mux.HandleFunc("GET /admin/api/settings", s.auth(s.handleGetSettings))
	mux.HandleFunc("PUT /admin/api/settings", s.formAware(s.auth(s.handlePutSettings)))
	mux.HandleFunc("POST /admin/api/password", s.formAware(s.auth(s.handleChangePassword)))

	mux.HandleFunc("GET /admin/api/audit", s.auth(s.handleAudit))
	mux.HandleFunc("GET /admin/api/version", s.auth(s.handleVersion))

	return withSecurityHeaders(mux)
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"version":    version.Version,
		"commit":     version.Commit,
		"build_date": version.BuildDate,
	})
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
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

type discardWriter struct {
	http.ResponseWriter
	status int
}

func (d *discardWriter) WriteHeader(code int) {
	d.status = code
}

func (d *discardWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

func (s *Server) formAware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
			next(w, r)
			return
		}
		ww := &discardWriter{ResponseWriter: w}
		next(ww, r)
		if ww.status == 0 {
			ww.status = http.StatusOK
		}
		http.Redirect(w, r, s.formRedirectTarget(r, ww.status == http.StatusOK), http.StatusSeeOther)
	}
}

func (s *Server) formRedirectTarget(r *http.Request, ok bool) string {
	flash := "success"
	if !ok {
		flash = "error"
	}
	base := "/admin"
	switch {
	case strings.HasPrefix(r.URL.Path, "/admin/api/login"):
		base = "/admin/login"
	case strings.HasPrefix(r.URL.Path, "/admin/api/scopes"):
		base = "/admin/scopes"
	case strings.HasPrefix(r.URL.Path, "/admin/api/notifications"):
		base = "/admin/notifications"
	case strings.HasPrefix(r.URL.Path, "/admin/api/requests"):
		base = "/admin/requests"
	case strings.HasPrefix(r.URL.Path, "/admin/api/settings"):
		base = "/admin/settings"
	case strings.HasPrefix(r.URL.Path, "/admin/api/password"):
		base = "/admin/settings"
	case strings.HasPrefix(r.URL.Path, "/admin/api/flush"):
		base = "/admin/settings"
	}
	return base + "?flash=" + flash
}

func activeSection(path string) string {
	switch {
	case path == "/admin" || path == "/admin/":
		return "home"
	case strings.HasPrefix(path, "/admin/requests"):
		return "requests"
	case strings.HasPrefix(path, "/admin/scopes"):
		return "scopes"
	case strings.HasPrefix(path, "/admin/notifications"):
		return "notifications"
	case strings.HasPrefix(path, "/admin/settings"):
		return "settings"
	case strings.HasPrefix(path, "/admin/audit"):
		return "audit"
	}
	return ""
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, page string, data map[string]any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if data == nil {
		data = map[string]any{}
	}
	if _, ok := data["Authed"]; !ok {
		data["Authed"] = true
	}
	data["Version"] = version.String()
	data["Active"] = activeSection(r.URL.Path)
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

func readJSONOrForm(r *http.Request, v any) error {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		return readJSON(r, v)
	}
	if err := r.ParseForm(); err != nil {
		return err
	}
	rv := reflect.ValueOf(v).Elem()
	t := rv.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		vals, ok := r.Form[name]
		if !ok || len(vals) == 0 {
			continue
		}
		fv := rv.Field(i)
		if !fv.CanSet() {
			continue
		}
		val := vals[0]
		switch fv.Kind() {
		case reflect.String:
			fv.SetString(val)
		case reflect.Bool:
			fv.SetBool(val == "on" || val == "1" || val == "true")
		case reflect.Int:
			if n, err := strconv.Atoi(val); err == nil {
				fv.SetInt(int64(n))
			}
		case reflect.Ptr:
			switch fv.Type().Elem().Kind() {
			case reflect.String:
				if val != "" {
					fv.Set(reflect.ValueOf(&val))
				}
			case reflect.Bool:
				b := val == "on" || val == "1" || val == "true"
				fv.Set(reflect.ValueOf(&b))
			case reflect.Int:
				if n, err := strconv.Atoi(val); err == nil {
					fv.Set(reflect.ValueOf(&n))
				}
			}
		}
	}
	return nil
}
