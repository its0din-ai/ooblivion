// Package auth handles password hashing, JWT issuance, and token revocation.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTooManyAttempts    = errors.New("too many attempts")
)

const (
	argonTime    = 2
	argonMemory  = 64 * 1024
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16
)

type Claims struct {
	Ver int `json:"ver"`
	jwt.RegisteredClaims
}

func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	encoded := struct {
		Time    uint32 `json:"t"`
		Memory  uint32 `json:"m"`
		Threads uint8  `json:"p"`
		Salt    string `json:"s"`
		Hash    string `json:"h"`
	}{
		Time:    argonTime,
		Memory:  argonMemory,
		Threads: argonThreads,
		Salt:    base64.RawStdEncoding.EncodeToString(salt),
		Hash:    base64.RawStdEncoding.EncodeToString(hash),
	}
	raw, err := json.Marshal(encoded)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func VerifyPassword(encoded, password string) (bool, error) {
	var params struct {
		Time    uint32 `json:"t"`
		Memory  uint32 `json:"m"`
		Threads uint8  `json:"p"`
		Salt    string `json:"s"`
		Hash    string `json:"h"`
	}
	if err := json.Unmarshal([]byte(encoded), &params); err != nil {
		return false, err
	}
	salt, err := base64.RawStdEncoding.DecodeString(params.Salt)
	if err != nil {
		return false, err
	}
	want, err := base64.RawStdEncoding.DecodeString(params.Hash)
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(password), salt, params.Time, params.Memory, params.Threads, uint32(len(want))) // #nosec G115 -- key length is fixed at 32 by HashPassword
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func IssueJWT(secret []byte, ttl time.Duration, tokenVersion int) (string, string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	jti := hex.EncodeToString(raw)
	now := time.Now()
	claims := Claims{
		Ver: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "admin",
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		return "", "", err
	}
	return signed, jti, nil
}

func ParseJWT(secret []byte, tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS512 {
			return nil, fmt.Errorf("unexpected signing method %v", t.Method)
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func DenylistAdd(handle *sql.DB, jti string, expiresAt time.Time) error {
	_, err := handle.Exec(
		"INSERT INTO jwt_denylist (jti, expires_at) VALUES (?, ?)",
		jti, expiresAt.UTC().Format(time.RFC3339),
	)
	return err
}

func DenylistCheck(handle *sql.DB, jti string) (bool, error) {
	var count int
	err := handle.QueryRow("SELECT COUNT(*) FROM jwt_denylist WHERE jti = ?", jti).Scan(&count)
	return count > 0, err
}

func DenylistPrune(handle *sql.DB) error {
	_, err := handle.Exec("DELETE FROM jwt_denylist WHERE expires_at < ?", time.Now().UTC().Format(time.RFC3339))
	return err
}

type RateLimiter struct {
	attempts map[string][]time.Time
	window   time.Duration
	max      int
}

func NewRateLimiter(max int, window time.Duration) *RateLimiter {
	return &RateLimiter{attempts: make(map[string][]time.Time), window: window, max: max}
}

func (r *RateLimiter) Allow(ip string) bool {
	now := time.Now()
	cutoff := now.Add(-r.window)
	recent := r.attempts[ip][:0]
	for _, t := range r.attempts[ip] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	r.attempts[ip] = recent
	if len(recent) >= r.max {
		return false
	}
	r.attempts[ip] = append(r.attempts[ip], now)
	return true
}

func ClientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	if real := r.Header.Get("X-Real-IP"); real != "" {
		return strings.TrimSpace(real)
	}
	host, _, err := splitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func splitHostPort(addr string) (string, string, error) {
	if i := strings.LastIndex(addr, ":"); i != -1 {
		return addr[:i], addr[i+1:], nil
	}
	return addr, "", nil
}

func NewCSRFToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

const CSRFHeader = "X-CSRF-Token"

func CSRFValid(r *http.Request) bool {
	cookie, err := r.Cookie("oob_csrf")
	if err != nil {
		return false
	}
	sent := r.Header.Get(CSRFHeader)
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(sent)) == 1
}
