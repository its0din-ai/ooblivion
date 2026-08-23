# ooblivion

Out-of-band HTTP capture and audit console. A single-user web app that records
every HTTP request that hits it, keeps the interesting ones, and alerts you on
the ones you care about.

Point any client at the listener (for example `https://acme.corp/data`), and
ooblivion stores the method, path, query string, all headers, body, source IP,
and Cloudflare country into SQLite. A password-protected console under
`/admin/` lets you inspect, search, save, and flush that data, define scope
rules, and configure Telegram alerts.

## Features

- Catch-all HTTP capture for any method, path, query, and body, including all
  client headers (stored verbatim).
- Visitor IP resolved from `Cf-Connecting-Ip` (Cloudflare), with
  `X-Forwarded-For`, `X-Real-IP`, and remote-address fallbacks; Cloudflare
  country (`Cf-Ipcountry`) shown as a flag.
- Mobile-first admin UI with light/dark theme (defaults to system), monospaced
  dashed-border design, hamburger nav with icons and active-state highlight.
- Single operator: password only (min 16 chars), no usernames.
- JWT HS512 sessions with server-side revocation on logout.
- Scope rules that auto-save matching requests into a protected bucket;
  enable/disable, edit, and delete straight from a table list.
- Retention: monthly auto-flush of unsaved data plus a manual flush on the
  Settings page; saved data persists until deleted.
- Cumulative capture counter (all-time total survives flushes).
- Telegram notifications with independent match rules, MarkdownV2 formatting,
  topic (thread) support, and a direct link to request details.
- Multi-domain deployment: only a configured admin host reaches `/admin/`;
  every other host is captured and answered with an empty 200.
- Request search with text, host (wildcard `*` supported), and method filters;
  pagination at 20 per page.
- Audit log with IP / action / failed-only filters, pagination, and a 3-month
  retention.
- SQLite storage with a persistent Docker volume.

## How it works

A single HTTP listener receives every request. Each request is stored, then
evaluated against scope rules and notification rules. If the request path
starts with `/admin/` AND the request host matches `ADMIN_HOST`, it is served
by the admin UI instead of being captured. Everything else, including
`/admin/` hits from other hosts, is captured and returns an empty 200.

Admin responses send `Cache-Control: no-store`, and static assets are
cache-busted with the build version, so deploys always serve fresh assets.

The app is intended to sit behind a reverse proxy that terminates TLS, such as
Caddy, typically in front of Cloudflare.

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
| `TELEGRAM_THREAD_ID` | Optional topic/thread id within the chat for supergroups with topics. |
| `LOG_LEVEL` | `debug`, `info`, `warn`, or `error`. |
| `MAX_BODY_BYTES` | Capture body size cap in bytes, default 1048576. |

`JWT_SECRET` and `ADMIN_PASSWORD` are checked at boot. Never commit `.env`.

## Docker deployment

Caddy runs on the host. Docker only runs the app and exposes port 8080 so the
host proxy can reach it. Build and deploy with the version embedded:

```sh
cp .env.example .env
# set ADMIN_HOST to the public admin domain, for example ooblivion.acme.corp
make up
```

`make up` derives the version from git tags (see Releases) and passes it to
Compose. If you use `docker compose` directly, supply the build args:

```sh
OOB_VERSION=v1.0 OOB_COMMIT=$(git rev-parse --short HEAD) docker compose up -d --build
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

*.acme.net {
    reverse_proxy 127.0.0.1:8080
}
```

Caddy terminates TLS and forwards `X-Forwarded-For`, `X-Forwarded-Proto`, and
`X-Forwarded-Host`, which the app uses to resolve the client IP, scheme, and
the admin host gate.

Wildcard zones need a wildcard certificate: Caddy obtains one via a DNS-01
challenge (requires a DNS provider plugin) or issues per-subdomain certs via
HTTP-01 once each subdomain's DNS resolves to this server. The app itself
captures any host.

The visitor IP is resolved with this precedence: `Cf-Connecting-Ip`
(Cloudflare) -> `X-Forwarded-For` -> `X-Real-IP` -> remote address. The
Cloudflare country header (`Cf-Ipcountry`) is stored per request and shown as a
flag on the requests list and detail page.

## Admin console

- Dashboard with a cumulative capture counter, saved/today counts, and quick
  actions. Manual flush lives on the Settings page only.
- Requests list with text search, host filter (wildcard `*` supported, e.g.
  `*.net`), method filter, pagination at 20 per page, clickable IPs that open
  ipinfo.io, and country flags.
- Request detail with raw method, path, query, headers, and body; source IP
  links to ipinfo.io; timestamps shown in local Indonesian (id-ID) format.
- Scopes: table list with enable/disable, edit (expands the form), and delete.
  Matching requests are auto-saved and never flushed.
- Notifications: independent Telegram match rules with enable/disable, test,
  edit, and delete.
- Settings: public URL base, retention window, auto-flush toggle, and password
  change.
- Audit: a log of admin actions with source IP, filterable by IP, action, and
  "failed only", paginated at 20 per page.
- Light/dark theme toggle in the topbar and on the login page.

## Retention and flush

- Auto-flush deletes unsaved requests older than the retention window
  (default 30 days, configurable) and runs on a schedule.
- Manual flush (Settings page) deletes all unsaved requests regardless of age.
- Saved requests are never auto-flushed; they are removed only by explicit
  user action.
- The dashboard total is cumulative and survives flushes.
- Audit log entries older than 3 months are pruned automatically.

## Telegram notifications

Create a notification rule (for example path contains `saved`). When a
captured request matches, the app sends a MarkdownV2 alert with the rule name,
an id-ID formatted time, the source IP, the endpoint in monospace (so it is
not auto-linked), and a view link:

```text
*OOBlivion Alert*

Alert Name: saved\-query
Time: 23/08/2026 19\.00\.33

IP: `198.51.100.77`
`GET acme.corp/data?saved=1`

View Url: [https://ooblivion.acme.corp/admin/requests/123](https://ooblivion.acme.corp/admin/requests/123)
```

If `TELEGRAM_THREAD_ID` is set, alerts are posted to that topic.

## Security

- Password hashing with Argon2id, minimum 16 characters.
- JWT HS512 with a secret of at least 64 bytes, enforced at boot.
- Logout revokes the token through a database denylist; password changes
  revoke all outstanding tokens.
- HttpOnly, SameSite=Strict cookies; the Secure flag is set behind TLS.
- All admin mutations require a valid JWT session; no CSRF or origin tokens
  (SameSite=Strict keeps the session browser-only). Keep the app behind a
  proxy that controls forwarded headers.
- Login rate limiting.
- Request bodies are capped and over-size bodies are flagged as truncated.
- Admin traffic is never captured, so operator credentials do not enter the
  request log.
- HTTP server timeouts and a data directory with restricted permissions.

## Project structure

```text
cmd/ooblivion/        entrypoint, server wiring, host gate
internal/config/      env loading and validation
internal/db/          SQLite open, WAL, embedded migrations
internal/models/      shared data structures
internal/auth/        argon2, JWT HS512, denylist
internal/capture/     request store, scopes, notification evaluation
internal/matcher/     rule matching engine
internal/scheduler/   flush and pruning jobs
internal/telegram/    Telegram sender worker (MarkdownV2)
internal/admin/       web UI and JSON API handlers
internal/logx/        leveled logger
internal/version/     build metadata injected via ldflags
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

Build with the version embedded (shown in the footer and `/admin/api/version`,
and used to version static asset URLs so deploys bust browser/CDN caches):

```sh
make build
```

Run the end-to-end test suite (self-starts an isolated instance on port 8180
with a temporary database, prints a pass/fail report, and cleans up):

```sh
./e2e.sh
```

## Releases

Versioning is tag-driven: `git describe --tags --always --dirty` (with the
commit count stripped) produces `v1.0-gabc123` - `major.minor` from the nearest
tag, `g<hash>` build suffix. The footer and `/admin/api/version` show it.

```sh
git tag v1.0 && git push origin v1.0   # release a version
make build                              # footer: v1.0 on the tag, v1.0-g<hash> after
make up                                 # docker deploy with the version embedded
```

## Notes

- The `resources/` directory holds agent-facing documents and is intentionally
  gitignored.
- `go.mod` and `go.sum` are tracked. `vendor/`, `data/`, and `.env` are not.
- `X-Forwarded-Host` is trusted for the admin host gate, so keep the app
  behind a proxy that controls that header.
