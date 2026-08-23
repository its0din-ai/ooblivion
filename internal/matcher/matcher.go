// Package matcher evaluates rules against captured request data.
package matcher

import (
	"regexp"
	"strings"
)

type Rule struct {
	MatchOn    string
	MatchType  string
	Pattern    string
	HeaderName string
}

type Subject struct {
	Method    string
	Host      string
	Path      string
	Query     string
	Body      string
	UserAgent string
	SourceIP  string
	Headers   map[string]string
}

func (s Subject) Field(name, headerName string) (string, bool) {
	switch name {
	case "host":
		return s.Host, s.Host != ""
	case "path":
		return s.Path, s.Path != ""
	case "query":
		return s.Query, s.Query != ""
	case "method":
		return s.Method, s.Method != ""
	case "header":
		if headerName == "" {
			return "", false
		}
		for key, value := range s.Headers {
			if strings.EqualFold(key, headerName) {
				return value, true
			}
		}
		return "", false
	case "body":
		return s.Body, s.Body != ""
	case "user_agent":
		return s.UserAgent, s.UserAgent != ""
	case "source_ip":
		return s.SourceIP, s.SourceIP != ""
	}
	return "", false
}

func Matches(s Subject, r Rule) bool {
	value, ok := s.Field(r.MatchOn, r.HeaderName)
	if !ok {
		return false
	}
	switch r.MatchType {
	case "exists":
		return value != ""
	case "equals":
		return strings.EqualFold(value, r.Pattern)
	case "prefix":
		return strings.HasPrefix(strings.ToLower(value), strings.ToLower(r.Pattern))
	case "suffix":
		return strings.HasSuffix(strings.ToLower(value), strings.ToLower(r.Pattern))
	case "regex":
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return false
		}
		return re.MatchString(value)
	default:
		return strings.Contains(strings.ToLower(value), strings.ToLower(r.Pattern))
	}
}
