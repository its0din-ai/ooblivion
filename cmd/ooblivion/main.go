// Command ooblivion runs the out-of-band HTTP capture and admin server.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"ooblivion/internal/admin"
	"ooblivion/internal/auth"
	"ooblivion/internal/capture"
	"ooblivion/internal/config"
	"ooblivion/internal/db"
	"ooblivion/internal/logx"
	"ooblivion/internal/matcher"
	"ooblivion/internal/models"
	"ooblivion/internal/scheduler"
	"ooblivion/internal/telegram"
	"ooblivion/internal/version"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		logx.New("error", os.Stderr, "ooblivion ").Fatalf("config: %v", err)
	}

	logger := logx.New(cfg.LogLevel, os.Stderr, "ooblivion ")
	logger.Infof("starting ooblivion %s", version.String())

	handle, err := db.Open(cfg.DatabasePath)
	if err != nil {
		logger.Fatalf("db: %v", err)
	}
	defer handle.Close()

	if err := ensureAuth(handle, cfg.AdminPassword); err != nil {
		logger.Fatalf("auth: %v", err)
	}

	tg := telegram.New(cfg.TelegramToken, cfg.TelegramChatID, cfg.TelegramThreadID, handle, logger)
	sched := scheduler.New(handle, 30, logger)
	classifier := capture.NewClassifier(handle, tg)

	app, err := admin.New(handle, cfg, tg, sched, logger)
	if err != nil {
		logger.Fatalf("admin: %v", err)
	}

	store := capture.NewStore(handle, cfg.MaxBodyBytes)

	root := &rootHandler{
		store:      store,
		classifier: classifier,
		admin:      app.Routes(),
		adminHosts: cfg.AdminHosts,
		catchAll:   http.HandlerFunc(catchAll),
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	sched.Start(ctx, 6*time.Hour)
	sched.RunOnce()

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           withCORS(root),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	go func() {
		logger.Infof("listening on %s", cfg.ListenAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}

func ensureAuth(handle *sql.DB, adminPassword string) error {
	var count int
	if err := handle.QueryRow("SELECT COUNT(*) FROM auth").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	if adminPassword == "" {
		return errors.New("auth table empty: set ADMIN_PASSWORD (min 16 chars) in .env and restart")
	}
	if len(adminPassword) < 16 {
		return errors.New("ADMIN_PASSWORD must be at least 16 chars")
	}
	hash, err := auth.HashPassword(adminPassword)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = handle.Exec(
		"INSERT INTO auth (id, password_hash, created_at, updated_at) VALUES (1, ?, ?, ?)",
		hash, now, now,
	)
	return err
}

type rootHandler struct {
	store      *capture.Store
	classifier *capture.Classifier
	admin      http.Handler
	adminHosts []string
	catchAll   http.Handler
}

func (h *rootHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/admin") && h.isAdminHost(r) {
		h.admin.ServeHTTP(w, r)
		return
	}

	req, err := h.store.Store(r)
	if err == nil {
		_ = h.classifier.Classify(req, subjectFromRequest(req))
	}
	h.catchAll.ServeHTTP(w, r)
}

func (h *rootHandler) isAdminHost(r *http.Request) bool {
	if len(h.adminHosts) == 0 {
		return true
	}
	for _, host := range []string{r.Host, r.Header.Get("X-Forwarded-Host")} {
		host = normalizeHost(host)
		for _, allowed := range h.adminHosts {
			if host == allowed {
				return true
			}
		}
	}
	return false
}

func normalizeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if i := strings.LastIndex(host, ":"); i != -1 && !strings.Contains(host[i+1:], "]") {
		host = host[:i]
	}
	return host
}

func subjectFromRequest(req models.Request) matcher.Subject {
	headers := map[string]string{}
	_ = json.Unmarshal([]byte(req.RequestHeaders), &headers)
	return matcher.Subject{
		Method:    req.Method,
		Host:      req.Host,
		Path:      req.Path,
		Query:     req.Query,
		Body:      req.Body,
		UserAgent: req.UserAgent,
		SourceIP:  req.SourceIP,
		Headers:   headers,
	}
}

func catchAll(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "*")
		h.Set("Access-Control-Allow-Headers", "*")
		h.Set("Access-Control-Expose-Headers", "*")
		next.ServeHTTP(w, r)
	})
}
