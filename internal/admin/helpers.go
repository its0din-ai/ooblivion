package admin

import (
	"database/sql"
	"net/http"

	"ooblivion/internal/auth"
)

type sqlNullInt64 = sql.NullInt64
type sqlNullString = sql.NullString

func authClientIP(r *http.Request) string {
	return auth.ClientIP(r)
}
