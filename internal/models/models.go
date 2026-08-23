// Package models defines the data structures used across the app.
package models

type Request struct {
	ID             int64
	Method         string
	Scheme         string
	Host           string
	Path           string
	Query          string
	HTTPVersion    string
	RequestHeaders string
	Body           string
	BodyTruncated  bool
	SourceIP       string
	RemoteAddr     string
	ForwardedFor   string
	UserAgent      string
	ContentType    string
	IPCountry      string
	Saved          bool
	ScopeID        *int64
	Notified       bool
	CreatedAt      string
}

type Scope struct {
	ID         int64
	Name       string
	MatchOn    string
	MatchType  string
	Pattern    string
	HeaderName *string
	Enabled    bool
	Priority   int
	CreatedAt  string
	UpdatedAt  string
}

type NotificationRule struct {
	ID         int64
	Name       string
	MatchOn    string
	MatchType  string
	Pattern    string
	HeaderName *string
	Enabled    bool
	CreatedAt  string
	UpdatedAt  string
}

type NotificationLog struct {
	ID        int64
	RuleID    *int64
	RequestID *int64
	Ok        bool
	Response  string
	SentAt    string
}

type AuditEntry struct {
	ID        int64
	Actor     string
	Action    string
	Detail    string
	IP        string
	CreatedAt string
}

type Stats struct {
	Total       int64  `json:"total"`
	Saved       int64  `json:"saved"`
	Today       int64  `json:"today"`
	ScopeCount  int64  `json:"scope_count"`
	RuleCount   int64  `json:"rule_count"`
	LastFlushAt string `json:"last_flush_at"`
}
