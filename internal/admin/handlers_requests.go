package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"ooblivion/internal/models"
)

type listResponse struct {
	Items   []models.Request `json:"items"`
	Page    int              `json:"page"`
	PerPage int              `json:"per_page"`
	Total   int64            `json:"total"`
	HasNext bool             `json:"has_next"`
}

func (s *Server) handleRequestsPage(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filters := requestFilters{
		q:       q.Get("q"),
		method:  q.Get("method"),
		host:    q.Get("host"),
		saved:   q.Get("saved"),
		scopeID: q.Get("scope_id"),
		page:    atoiDefault(q.Get("page"), 1),
	}
	items, total, err := s.queryRequests(filters)
	if err != nil {
		s.logger.Errorf("query requests: %v", err)
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	pagination := s.buildPagination(filters, total)
	if pagination["Page"].(int) != filters.page {
		filters.page = pagination["Page"].(int)
		items, total, err = s.queryRequests(filters)
		if err != nil {
			s.logger.Errorf("query requests: %v", err)
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
	}
	hosts, herr := s.distinctHosts()
	if herr != nil {
		s.logger.Errorf("query hosts: %v", herr)
	}
	s.render(w, r, "requests", map[string]any{
		"Title":      "Requests",
		"Items":      items,
		"Total":      total,
		"Methods":    []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "TRACE", "CONNECT"},
		"Hosts":      hosts,
		"Q":          filters.q,
		"Method":     filters.method,
		"Host":       filters.host,
		"Saved":      filters.saved,
		"Pagination": pagination,
	})
}

type pageLink struct {
	Number int
	Active bool
	Href   string
}

func (s *Server) buildPagination(f requestFilters, total int64) map[string]any {
	perPage := f.perPage()
	pages := int((total + int64(perPage) - 1) / int64(perPage))
	if pages < 1 {
		pages = 1
	}
	page := f.page
	if page > pages {
		page = pages
	}

	href := func(p int) string {
		return fmt.Sprintf("/admin/requests?q=%s&method=%s&saved=%s&host=%s&page=%d",
			url.QueryEscape(f.q), url.QueryEscape(f.method), url.QueryEscape(f.saved), url.QueryEscape(f.host), p)
	}

	start := page - 2
	if start < 1 {
		start = 1
	}
	end := start + 4
	if end > pages {
		end = pages
		start = end - 4
		if start < 1 {
			start = 1
		}
	}
	numbers := []pageLink{}
	for n := start; n <= end; n++ {
		numbers = append(numbers, pageLink{Number: n, Active: n == page, Href: href(n)})
	}

	return map[string]any{
		"Page":      page,
		"Pages":     pages,
		"HasPrev":   page > 1,
		"HasNext":   page < pages,
		"FirstHref": href(1),
		"PrevHref":  href(maxInt(page-1, 1)),
		"NextHref":  href(minInt(page+1, pages)),
		"LastHref":  href(pages),
		"Numbers":   numbers,
	}
}

func (s *Server) distinctHosts() ([]string, error) {
	rows, err := s.db.Query("SELECT DISTINCT host FROM requests WHERE host != '' ORDER BY host ASC LIMIT 200")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hosts := []string{}
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, err
		}
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *Server) handleDetailPage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	req, err := s.getRequest(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var headers map[string]string
	_ = json.Unmarshal([]byte(req.RequestHeaders), &headers)
	s.render(w, r, "detail", map[string]any{
		"Title":   fmt.Sprintf("Request #%d", id),
		"Request": req,
		"Headers": headers,
	})
}

func (s *Server) handleListRequests(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filters := requestFilters{
		q:       q.Get("q"),
		method:  q.Get("method"),
		host:    q.Get("host"),
		saved:   q.Get("saved"),
		scopeID: q.Get("scope_id"),
		page:    atoiDefault(q.Get("page"), 1),
	}
	items, total, err := s.queryRequests(filters)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
		return
	}
	writeJSON(w, http.StatusOK, listResponse{
		Items:   items,
		Page:    filters.page,
		PerPage: filters.perPage(),
		Total:   total,
		HasNext: int64(filters.page)*int64(filters.perPage()) < total,
	})
}

func (s *Server) handleGetRequest(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad id"})
		return
	}
	req, err := s.getRequest(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, req)
}

func (s *Server) handleSaveRequest(w http.ResponseWriter, r *http.Request) {
	s.toggleSaved(w, r, true)
}

func (s *Server) handleUnsaveRequest(w http.ResponseWriter, r *http.Request) {
	s.toggleSaved(w, r, false)
}

func (s *Server) toggleSaved(w http.ResponseWriter, r *http.Request, saved bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad id"})
		return
	}
	query := "UPDATE requests SET saved = ?, scope_id = NULL WHERE id = ?"
	if saved {
		query = "UPDATE requests SET saved = ? WHERE id = ?"
	}
	if _, err := s.db.Exec(query, boolToInt64(saved), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
		return
	}
	s.logAudit("save_request", fmt.Sprintf("request %d saved=%v", id, saved), authClientIP(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDeleteRequest(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "bad id"})
		return
	}
	if _, err := s.db.Exec("DELETE FROM requests WHERE id = ?", id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
		return
	}
	s.logAudit("delete_request", fmt.Sprintf("request %d deleted", id), authClientIP(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleBulkDelete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if err := readJSON(r, &body); err != nil || len(body.IDs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid body"})
		return
	}
	placeholders := strings.Repeat("?,", len(body.IDs)-1) + "?"
	args := make([]any, len(body.IDs))
	for i, id := range body.IDs {
		args[i] = id
	}
	res, err := s.db.Exec("DELETE FROM requests WHERE id IN ("+placeholders+")", args...) // #nosec G202 -- placeholders are only generated ?, from the id count; values are bound
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
		return
	}
	n, _ := res.RowsAffected()
	s.logAudit("bulk_delete", fmt.Sprintf("deleted %d requests", n), authClientIP(r))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": n})
}

func (s *Server) handleFlush(w http.ResponseWriter, r *http.Request) {
	count, err := s.sched.Flush(true, authClientIP(r))
	if err != nil {
		s.logger.Errorf("flush: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": count})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	var total, saved, today int64
	_ = s.db.QueryRow("SELECT total_captured FROM stats WHERE id = 1").Scan(&total)
	_ = s.db.QueryRow("SELECT COUNT(*) FROM requests WHERE saved = 1").Scan(&saved)
	start := time.Now().UTC().Format("2006-01-02T00:00:00Z")
	_ = s.db.QueryRow("SELECT COUNT(*) FROM requests WHERE created_at >= ?", start).Scan(&today)

	var scopeCount, ruleCount int64
	_ = s.db.QueryRow("SELECT COUNT(*) FROM scopes").Scan(&scopeCount)
	_ = s.db.QueryRow("SELECT COUNT(*) FROM notification_rules").Scan(&ruleCount)

	writeJSON(w, http.StatusOK, models.Stats{
		Total:       total,
		Saved:       saved,
		Today:       today,
		ScopeCount:  scopeCount,
		RuleCount:   ruleCount,
		LastFlushAt: s.sched.LastFlushAt(),
	})
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(
		`SELECT id, actor, action, detail, ip, created_at FROM audit_log ORDER BY id DESC LIMIT 200`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server error"})
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
	writeJSON(w, http.StatusOK, items)
}

type requestFilters struct {
	q       string
	method  string
	host    string
	saved   string
	scopeID string
	page    int
}

func (f requestFilters) perPage() int {
	return 50
}

func (s *Server) queryRequests(f requestFilters) ([]models.Request, int64, error) {
	where := []string{}
	args := []any{}
	if f.q != "" {
		like := "%" + f.q + "%"
		where = append(where, "(path LIKE ? OR query LIKE ? OR body LIKE ? OR host LIKE ? OR user_agent LIKE ?)")
		args = append(args, like, like, like, like, like)
	}
	if f.method != "" {
		where = append(where, "method = ?")
		args = append(args, f.method)
	}
	if f.host != "" {
		if strings.Contains(f.host, "*") {
			pattern := f.host
			pattern = strings.ReplaceAll(pattern, `\`, `\\`)
			pattern = strings.ReplaceAll(pattern, "%", `\%`)
			pattern = strings.ReplaceAll(pattern, "_", `\_`)
			pattern = strings.ReplaceAll(pattern, "*", "%")
			where = append(where, "host LIKE ? ESCAPE '\\'")
			args = append(args, pattern)
		} else {
			where = append(where, "host = ?")
			args = append(args, f.host)
		}
	}
	if f.saved == "1" || f.saved == "0" {
		where = append(where, "saved = ?")
		args = append(args, f.saved)
	}
	if f.scopeID != "" {
		where = append(where, "scope_id = ?")
		args = append(args, f.scopeID)
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int64
	if err := s.db.QueryRow("SELECT COUNT(*) FROM requests"+clause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (f.page - 1) * f.perPage()
	limit := f.perPage()
	args = append(args, limit, offset)
	rows, err := s.db.Query(
		`SELECT id, method, scheme, host, path, query, http_version, request_headers, body,
		 body_truncated, source_ip, remote_addr, forwarded_for, user_agent, content_type, COALESCE(ip_country, ''),
		 saved, scope_id, notified, created_at
		 FROM requests`+clause+` ORDER BY id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []models.Request{}
	for rows.Next() {
		var req models.Request
		var truncated, saved, notified int
		var scopeID sqlNullInt64
		if err := rows.Scan(
			&req.ID, &req.Method, &req.Scheme, &req.Host, &req.Path, &req.Query, &req.HTTPVersion,
			&req.RequestHeaders, &req.Body, &truncated, &req.SourceIP, &req.RemoteAddr,
			&req.ForwardedFor, &req.UserAgent, &req.ContentType, &req.IPCountry, &saved, &scopeID, &notified,
			&req.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		req.BodyTruncated = truncated == 1
		req.Saved = saved == 1
		req.Notified = notified == 1
		if scopeID.Valid {
			req.ScopeID = &scopeID.Int64
		}
		items = append(items, req)
	}
	return items, total, rows.Err()
}

func (s *Server) getRequest(id int64) (models.Request, error) {
	var req models.Request
	var truncated, saved, notified int
	var scopeID sqlNullInt64
	err := s.db.QueryRow(
		`SELECT id, method, scheme, host, path, query, http_version, request_headers, body,
		 body_truncated, source_ip, remote_addr, forwarded_for, user_agent, content_type, COALESCE(ip_country, ''),
		 saved, scope_id, notified, created_at FROM requests WHERE id = ?`, id,
	).Scan(
		&req.ID, &req.Method, &req.Scheme, &req.Host, &req.Path, &req.Query, &req.HTTPVersion,
		&req.RequestHeaders, &req.Body, &truncated, &req.SourceIP, &req.RemoteAddr,
		&req.ForwardedFor, &req.UserAgent, &req.ContentType, &req.IPCountry, &saved, &scopeID, &notified,
		&req.CreatedAt,
	)
	if err != nil {
		return req, err
	}
	req.BodyTruncated = truncated == 1
	req.Saved = saved == 1
	req.Notified = notified == 1
	if scopeID.Valid {
		req.ScopeID = &scopeID.Int64
	}
	return req, nil
}

func atoiDefault(s string, def int) int {
	if v, err := strconv.Atoi(s); err == nil && v > 0 {
		return v
	}
	return def
}

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
