// Package capture stores incoming HTTP requests and applies scopes and notification rules.
package capture

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ooblivion/internal/models"
)

type Store struct {
	db      *sql.DB
	maxBody int64
}

func NewStore(db *sql.DB, maxBody int64) *Store {
	return &Store{db: db, maxBody: maxBody}
}

func (s *Store) Store(r *http.Request) (models.Request, error) {
	headers := make(map[string]string)
	for name, values := range r.Header {
		headers[name] = strings.Join(values, ", ")
	}
	headersJSON, _ := json.Marshal(headers)

	body, truncated := s.readBody(r)

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}

	now := time.Now().UTC().Format(time.RFC3339)
	req := models.Request{
		Method:         r.Method,
		Scheme:         scheme,
		Host:           r.Host,
		Path:           r.URL.Path,
		Query:          r.URL.RawQuery,
		HTTPVersion:    r.Proto,
		RequestHeaders: string(headersJSON),
		Body:           body,
		BodyTruncated:  truncated,
		SourceIP:       clientIP(r),
		RemoteAddr:     r.RemoteAddr,
		ForwardedFor:   forwardedIPs(r),
		UserAgent:      r.Header.Get("User-Agent"),
		ContentType:    r.Header.Get("Content-Type"),
		CreatedAt:      now,
	}

	res, err := s.db.Exec(
		`INSERT INTO requests (method, scheme, host, path, query, http_version, request_headers,
		 body, body_truncated, source_ip, remote_addr, forwarded_for, user_agent, content_type, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.Method, req.Scheme, req.Host, req.Path, req.Query, req.HTTPVersion, req.RequestHeaders,
		req.Body, boolToInt(req.BodyTruncated), req.SourceIP, req.RemoteAddr, req.ForwardedFor,
		req.UserAgent, req.ContentType, req.CreatedAt,
	)
	if err != nil {
		return req, fmt.Errorf("store request: %w", err)
	}
	req.ID, err = res.LastInsertId()
	if err != nil {
		return req, err
	}
	return req, nil
}

func (s *Store) readBody(r *http.Request) (string, bool) {
	if r.Body == nil {
		r.Body = http.NoBody
		return "", false
	}
	limited := io.LimitReader(r.Body, s.maxBody+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		r.Body = http.NoBody
		return "", false
	}
	truncated := int64(len(raw)) > s.maxBody
	if truncated {
		raw = raw[:s.maxBody]
	}
	r.Body = io.NopCloser(bytes.NewReader(raw))
	return string(raw), truncated
}

func clientIP(r *http.Request) string {
	if cf := r.Header.Get("CF-Connecting-IP"); cf != "" {
		return strings.TrimSpace(cf)
	}
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	if real := r.Header.Get("X-Real-IP"); real != "" {
		return strings.TrimSpace(real)
	}
	if i := strings.LastIndex(r.RemoteAddr, ":"); i != -1 {
		return r.RemoteAddr[:i]
	}
	return r.RemoteAddr
}

func forwardedIPs(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return fwd
	}
	return r.Header.Get("X-Real-IP")
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
