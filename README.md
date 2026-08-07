# Organizer

A self-hosted tool for martial-arts trainers to organize the people they train.
Discipline-agnostic (not tied to BJJ), unified by the concept of graduation.
The first capability is a shared roster of athletes and their graduation history.

> **Status: v1 built, not yet deployed.** Persistence, authentication, athlete
> management and promotions are done, along with a sortable and filterable
> roster, belt graphics, a bilingual UI and CSV import. Deployment is the one
> remaining v1 issue. Work is tracked under [`.scratch/`](.scratch/) — see
> [`docs/agents/issue-tracker.md`](docs/agents/issue-tracker.md).

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
- **Belt graphics** — a rank renders as an inline SVG belt where its colour is
  known, with the rank name always available as the accessible label.
- **Bilingual UI** — German and English, chosen per trainer account.
- **CSV athlete import** via the CLI.

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
cmd/organizer/      Entry point: HTTP server + CLI subcommands
internal/
  config/           Environment-variable configuration
  web/              Router, handlers, html/templates, static assets (HTMX, CSS)
  store/            Data-access layer, goose migrations, grading-system seeding
  auth/             Password hashing
  i18n/             Embedded translation catalogs (de, en)
docs/adr/           Architecture Decision Records
docs/agents/        Conventions for agents: issue tracker, triage, domain docs
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
go build ./...                             # compile
go vet ./...                               # static checks
go test ./...                              # run tests
go test ./internal/web/ -fuzz FuzzReturnPath -fuzztime 60s   # hunt new fuzz inputs
go test -coverprofile=coverage.out ./...   # coverage profile (git-ignored)
go tool cover -func=coverage.out           # per-function coverage, and the total
go tool cover -html=coverage.out           # the same, as annotated source

go run honnef.co/go/tools/cmd/staticcheck@latest ./...   # deeper static analysis
go run golang.org/x/vuln/cmd/govulncheck@latest ./...    # known CVEs, reachable ones only
```

The last two are `go run` rather than installed binaries so that the toolchain
stays the only dependency, the same reason the coverage commands above are bare
`go` commands. `@latest` is deliberate: without CI there is no build to make
reproducible, and a pinned tools dependency would go stale unnoticed.

Nothing automates `govulncheck`, and it is the one check whose answer changes
while the code sits still, so run it yourself — always before a deploy.
[`docs/agents/analysis.md`](docs/agents/analysis.md) has the rest: why the list is
this short, which analysers were tried and rejected, and how to react when a scan
reports standard-library findings.

Record coverage as two absolute counts — statements covered of statements total —
rather than the percentage, which a refactor that deletes untested code raises
without adding a test. The profile carries the counts the percentage is derived
from: `awk 'NR>1 {t+=$2; if ($3>0) c+=$2} END {print c, "of", t}' coverage.out`.
There is no threshold and no CI gate.

Contributions follow the issue tracker under `.scratch/` and the conventions in
[`docs/agents/`](docs/agents/). SQL must stay portable (no SQLite-specific
features) per ADR-0002.

## License

[MIT](LICENSE) © 2026 Markus Thiede
