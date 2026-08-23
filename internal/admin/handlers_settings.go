package admin

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"ooblivion/internal/models"
	"ooblivion/internal/scheduler"
)

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "dashboard", map[string]any{"Title": "Dashboard"})
}

func (s *Server) handleSettingsPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "settings", map[string]any{
		"Title":            "Settings",
		"PublicURL":        scheduler.ReadSetting(s.db, "public_url"),
		"RetentionDays":    s.sched.RetentionDays(),
		"AutoFlush":        scheduler.ReadSetting(s.db, "auto_flush_enabled") != "0",
		"TelegramToken":    s.cfg.TelegramToken != "",
		"TelegramChatID":   s.cfg.TelegramChatID,
		"TelegramThreadID": s.cfg.TelegramThreadID,
		"LastFlushAt":      s.sched.LastFlushAt(),
	})
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"public_url":         scheduler.ReadSetting(s.db, "public_url"),
		"retention_days":     s.sched.RetentionDays(),
		"auto_flush_enabled": scheduler.ReadSetting(s.db, "auto_flush_enabled") != "0",
		"telegram_enabled":   s.cfg.TelegramToken != "" && s.cfg.TelegramChatID != "",
		"last_flush_at":      s.sched.LastFlushAt(),
	})
}

func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PublicURL        *string `json:"public_url"`
		RetentionDays    *int    `json:"retention_days"`
		AutoFlushEnabled *bool   `json:"auto_flush_enabled"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid body"})
		return
	}
	if body.RetentionDays != nil && (*body.RetentionDays <= 0 || *body.RetentionDays > 3650) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "retention_days out of range"})
		return
	}
	if body.PublicURL != nil {
		if err := scheduler.WriteSetting(s.db, "public_url", *body.PublicURL); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
			return
		}
	}
	if body.RetentionDays != nil {
		if err := scheduler.WriteSetting(s.db, "retention_days", strconv.Itoa(*body.RetentionDays)); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
			return
		}
	}
	if body.AutoFlushEnabled != nil {
		val := "0"
		if *body.AutoFlushEnabled {
			val = "1"
		}
		if err := scheduler.WriteSetting(s.db, "auto_flush_enabled", val); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
			return
		}
	}
	s.logAudit("settings_update", "settings changed", authClientIP(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAuditPage(w http.ResponseWriter, r *http.Request) {
	const perPage = 20
	q := r.URL.Query()
	page := atoiDefault(q.Get("page"), 1)
	fip := q.Get("ip")
	fact := q.Get("action")
	ffailed := q.Get("failed")

	where := []string{}
	args := []any{}
	if fip != "" {
		where = append(where, "ip LIKE ?")
		args = append(args, "%"+fip+"%")
	}
	if fact != "" {
		where = append(where, "action = ?")
		args = append(args, fact)
	}
	if ffailed == "on" {
		where = append(where, "action LIKE '%failed%'")
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int64
	if err := s.db.QueryRow("SELECT COUNT(*) FROM audit_log"+clause, args...).Scan(&total); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	pagination := buildPagination(page, perPage, total, func(p int) string {
		return fmt.Sprintf("/admin/audit?ip=%s&action=%s&failed=%s&page=%d",
			url.QueryEscape(fip), url.QueryEscape(fact), url.QueryEscape(ffailed), p)
	})

	queryArgs := append(args, perPage, (pagination.Page-1)*perPage)
	rows, err := s.db.Query(
		`SELECT id, actor, action, detail, ip, created_at FROM audit_log`+clause+
			` ORDER BY id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	items := []models.AuditEntry{}
	for rows.Next() {
		var e models.AuditEntry
		if err := rows.Scan(&e.ID, &e.Actor, &e.Action, &e.Detail, &e.IP, &e.CreatedAt); err != nil {
			continue
		}
		items = append(items, e)
	}

	actions := []string{}
	if actRows, err := s.db.Query("SELECT DISTINCT action FROM audit_log ORDER BY action"); err == nil {
		defer actRows.Close()
		for actRows.Next() {
			var a string
			if actRows.Scan(&a) == nil {
				actions = append(actions, a)
			}
		}
	}

	s.render(w, r, "audit", map[string]any{
		"Title":      "Audit",
		"Items":      items,
		"Pagination": pagination,
		"IP":         fip,
		"Action":     fact,
		"Failed":     ffailed == "on",
		"Actions":    actions,
	})
}
