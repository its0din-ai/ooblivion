# ooblivion

Out-of-band HTTP capture and audit console. A single-user web app that records
every HTTP request that hits it, keeps the interesting ones, and alerts you on
the ones you care about.

Point any client at the listener (for example `https://acme.corp/data`), and
ooblivion stores the method, path, query string, all headers, body, and source
IP into SQLite. A password-protected console under `/admin/` lets you inspect,
search, save, and flush that data, define scope rules, and configure Telegram
alerts.

## Features

- Catch-all HTTP capture for any method, path, query, and body, including all
  client headers.
- Mobile-first admin UI with a dark, monospaced, dashed-border theme.
- Single operator: password only (min 16 chars), no usernames.
- JWT HS512 sessions with server-side revocation on logout.
- Scope rules that auto-save matching requests into a protected bucket.
- Monthly retention flush of unsaved data; saved data persists until deleted.
- Telegram notifications with independent match rules and a direct link to the
  request details.
- Multi-domain deployment: only a configured admin host reaches `/admin/`;
  every other host is captured and answered with an empty 200.
- SQLite storage with a persistent Docker volume.

## How it works

A single HTTP listener receives every request. Each request is stored, then
evaluated against scope rules and notification rules. If the request path
starts with `/admin/` AND the request host matches `ADMIN_HOST`, it is served
by the admin UI instead of being captured. Everything else, including
`/admin/` hits from other hosts, is captured and returns an empty 200.

The app is intended to sit behind a reverse proxy that terminates TLS, such as
Caddy.

## Requirements

- Go 1.26 or newer to build from source.
- Docker and Docker Compose for container deployment (optional).
- A Caddy (or any) reverse proxy for public TLS deployment.

## Quick start (local)

```sh
cp .env.example .env
# fill JWT_SECRET (min 64 bytes) and ADMIN_PASSWORD (min 16 chars)
go build -o bin/ooblivion ./cmd/ooblivion
./bin/ooblivion
```

Open http://127.0.0.1:8080/admin, log in, and capture a test request:

```sh
curl http://127.0.0.1:8080/anything?token=abc123
```

## Configuration

All configuration lives in `.env`. Copy `.env.example` and fill it in. Key
values:

| Key | Purpose |
| --- | --- |
| `OOB_LISTEN_ADDR` | Listener address, for example `:8080` or `127.0.0.1:8082`. |
| `ADMIN_HOST` | Comma-separated hostnames allowed to reach `/admin/`. Empty means any host. |
| `DATABASE_PATH` | SQLite file path, for example `data/ooblivion.db`. |
| `JWT_SECRET` | HS512 signing secret, must be at least 64 bytes. |
| `ADMIN_PASSWORD` | Initial admin password (min 16 chars), used only on first boot. |
| `TELEGRAM_BOT_TOKEN` | Telegram bot token. Empty disables notifications. |
| `TELEGRAM_CHAT_ID` | Telegram chat id that receives alerts. |
| `LOG_LEVEL` | `debug`, `info`, `warn`, or `error`. |
| `MAX_BODY_BYTES` | Capture body size cap in bytes, default 1048576. |

`JWT_SECRET` and `ADMIN_PASSWORD` are checked at boot. Never commit `.env`.

## Docker deployment

Caddy runs on the host. Docker only runs the app and exposes port 8080 so the
host proxy can reach it.

```sh
cp .env.example .env
# set ADMIN_HOST to the public admin domain, for example ooblivion.acme.corp
OOB_VERSION=$(git rev-parse --short HEAD) docker compose up -d --build
```

The SQLite database lives in the `ooblivion_data` Docker volume and persists
across rebuilds and restarts.

## Reverse proxy (Caddy, multi-domain)

See `Caddyfile.example`. Route all your capture domains and the admin domain to
the app:

```text
ooblivion.acme.corp {
    reverse_proxy 127.0.0.1:8080
}

*.acme.corp {
    reverse_proxy 127.0.0.1:8080
}
```

Caddy terminates TLS and forwards `X-Forwarded-For`, `X-Forwarded-Proto`, and
`X-Forwarded-Host`, which the app uses to resolve the client IP, scheme, and
the admin host gate.

The visitor IP is resolved with this precedence: `Cf-Connecting-Ip`
(Cloudflare) → `X-Forwarded-For` → `X-Real-IP` → remote address.

The Cloudflare country header (`Cf-Ipcountry`) is stored per request and shown
as a flag on the requests list and detail page.

## Admin console

- Dashboard with capture counters and quick actions.
- Requests list with search, filter, and compact pagination.
- Request detail with raw method, path, query, headers, and body.
- Scopes: rules that auto-save matching requests so they are never flushed.
- Notifications: independent Telegram match rules with a test button.
- Settings: retention window, auto-flush toggle, public URL base, and password
  change.
- Audit: a log of admin actions with source IP.

## Retention and flush

- Auto-flush deletes unsaved requests older than the retention window
  (default 30 days, configurable) and runs on a schedule.
- Manual flush deletes all unsaved requests regardless of age.
- Saved requests are never auto-flushed; they are removed only by explicit
  user action.

## Telegram notifications

Create a notification rule (for example path contains `saved`). When a
captured request matches, the app sends a concise alert with the method, host,
path, timestamp, and a view link to the request detail:

```text
ooblivion alert - saved-query
GET acme.corp/data?saved=1
2026-08-23T04:00:00Z
View: https://ooblivion.acme.corp/admin/requests/123
```

## Security

- Password hashing with Argon2id, minimum 16 characters.
- JWT HS512 with a secret of at least 64 bytes, enforced at boot.
- Logout revokes the token through a database denylist; password changes
  revoke all outstanding tokens.
- HttpOnly, SameSite=Strict cookies; the Secure flag is set behind TLS.
- CSRF protection on all mutating admin endpoints.
- Login rate limiting.
- Request bodies are capped and over-size bodies are flagged as truncated.
- Admin traffic is never captured, so operator credentials do not enter the
  request log.

## Project structure

```text
cmd/ooblivion/        entrypoint, server wiring, host gate
internal/config/      env loading and validation
internal/db/          SQLite open, WAL, embedded migrations
internal/models/      shared data structures
internal/auth/        argon2, JWT HS512, denylist, CSRF
internal/capture/     request store, scopes, notification evaluation
internal/matcher/     rule matching engine
internal/scheduler/   flush and pruning jobs
internal/telegram/    Telegram sender worker
internal/admin/       web UI and JSON API handlers
internal/web/         embedded templates and static assets
resources/            LLM agent docs (SPEC, PHASE, AGENTIC), gitignored
data/                 local SQLite files, gitignored
```

## Development

```sh
go build ./...
go vet ./...
staticcheck ./...
gofmt -l .
```

Build with the git commit hash embedded (shown in the footer and
`/admin/api/version`, and used to version static asset URLs so deploys bust
browser/CDN caches):

```sh
make build
```

Run the end-to-end test suite (self-starts an isolated instance on port 8180
with a temporary database, prints a pass/fail report, and cleans up):

```sh
./e2e.sh
```

## Notes

- The `resources/` directory holds agent-facing documents and is intentionally
  gitignored.
- `go.mod` and `go.sum` are tracked. `vendor/`, `data/`, and `.env` are not.
- `X-Forwarded-Host` is trusted for the admin host gate, so keep the app
  behind a proxy that controls that header.
