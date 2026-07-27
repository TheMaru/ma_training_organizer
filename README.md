# Organizer

A self-hosted tool for martial-arts trainers to organize the people they train.
Discipline-agnostic (not tied to BJJ), unified by the concept of graduation.
The first capability is a shared roster of athletes and their graduation history.

> **Status: early development.** The project skeleton (HTTP server, templating,
> static assets) is in place. Persistence, authentication, athlete management,
> promotions and deployment are specified and tracked under
> [`.scratch/v1-athlete-roster/`](.scratch/v1-athlete-roster/) and not yet built.

## What it does (v1 goal)

Trainers log in and maintain a shared roster of athletes and their graduations:

- **Trainer login** with session auth and self-service password change. Accounts
  and password resets are provisioned via CLI — no self-registration, no email.
- **Athlete roster** — create, edit and delete athletes (`firstName`,
  `lastName`, `birthDate`, `joinedOn`, `notes`), list sortable by any column
  including the derived current rank.
- **Graduation** — record promotions (grading system + rank + date) and show the
  derived current rank plus full history per athlete.
- **Seeded grading systems** — "BJJ Kids" and "BJJ Adult", stripes included.

See [`.scratch/v1-athlete-roster/spec.md`](.scratch/v1-athlete-roster/spec.md)
for the full v1 specification and scope boundaries.

## Domain

The vocabulary is deliberate — see [`CONTEXT.md`](CONTEXT.md) for the glossary.
In short: a **Promotion** records an **Athlete** reaching a **Rank** on a date; a
**GradingSystem** is an ordered set of ranks; the current rank is *derived* (the
most recent promotion by date), never stored as a field. Trainers are the only
account type. Key decisions are recorded as ADRs in [`docs/adr/`](docs/adr/).

## Tech stack

Chosen for the cheapest, simplest self-hosted deployment (see
[ADR-0002](docs/adr/0002-stack-go-sqlite-htmx-fly.md)):

- **Go** — compiles to a single static binary.
- **SQLite** — one file on a persistent volume; SQL kept portable so a later move
  to Postgres stays cheap.
- **Server-rendered `html/template` + [HTMX](https://htmx.org)** — no build step,
  no JSON API boundary. HTMX is vendored as a static asset (no CDN).
- **Sessions** via `alexedwards/scs` (SQLite-backed) + `alexedwards/argon2id`.
- **Fly.io** (EU region) with a volume and Litestream backups.

Everything ships as one binary plus one SQLite file.

## Project layout

```
cmd/organizer/      Entry point: HTTP server + (planned) CLI subcommands
internal/
  config/           Environment-variable configuration
  web/              Router, handlers, html/templates, static assets (HTMX, CSS)
  db/               SQLite open + goose migrations        (issue 02)
  store/            Data-access layer                     (issue 02+)
  seed/             Grading-system seeding                (issue 03)
  auth/             Password hashing, sessions            (issue 04)
docs/adr/           Architecture Decision Records
.scratch/           Feature specs and issue tracker (see docs/agents/)
```

## Getting started

Requires **Go 1.25+**.

```sh
git clone git@github.com:TheMaru/ma_training_organizer.git
cd ma_training_organizer
go run ./cmd/organizer
```

The server listens on `:8080` by default. Visit
[http://localhost:8080](http://localhost:8080); the health endpoint is at
`/healthz`.

### Configuration

All configuration is via environment variables:

| Variable            | Default        | Description                                   |
| ------------------- | -------------- | --------------------------------------------- |
| `ORGANIZER_ADDR`    | `:8080`        | TCP address the HTTP server listens on        |
| `ORGANIZER_DB_PATH` | `organizer.db` | Path to the SQLite database file              |
| `ORGANIZER_SECURE`  | `false`        | Mark session cookies `Secure` (HTTPS-only)    |

## Development

```sh
go build ./...    # compile
go vet ./...      # static checks
go test ./...     # run tests
```

Contributions follow the issue tracker under `.scratch/` and the conventions in
[`docs/agents/`](docs/agents/). SQL must stay portable (no SQLite-specific
features) per ADR-0002.

## License

[MIT](LICENSE) © 2026 Markus Thiede
