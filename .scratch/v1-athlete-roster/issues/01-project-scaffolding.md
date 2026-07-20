# 01 — Project scaffolding

Status: ready-for-agent

Set up the Go project skeleton so features have a home. No domain features yet.

## Scope

- Go module + layout (`cmd/`, `internal/`).
- HTTP server (stdlib `net/http`, chi router acceptable).
- `html/template` rendering with a base layout; HTMX vendored as a static asset (no CDN, keeps single-artifact deploy).
- Static asset serving; config via environment variables.
- A healthcheck endpoint.

## Acceptance

- `go run ./...` starts the server.
- Serves a base layout page with HTMX loaded and a static asset resolving.
