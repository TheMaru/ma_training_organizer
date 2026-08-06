# 07 — Deployment: Fly.io (EU)

Status: ready-for-human
Blocked by: 01

Deploy the single binary + SQLite to Fly.io in an EU region (ADR-0002, ADR-0003). Marked `ready-for-human` because it needs a Fly account, secrets, object-storage credentials, and DNS.

## Scope

- Fly.io app config (`fly.toml`), **EU region** (e.g. `fra`/`ams`).
- Persistent volume mounted for the SQLite file; machine auto-stop on idle.
- HTTPS (platform-managed).
- Litestream streaming the DB to EU object storage, with **~30-day retention** so deletions age out of backups (ADR-0003).
- Secrets management for app config.
- Domain + DNS.

## Acceptance

- App reachable over HTTPS from an EU region.
- Data survives a redeploy (volume persists).
- Backups are produced and old ones pruned at the retention window.
- `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` reports no reachable
  vulnerability, run **immediately before** the deploy rather than trusted from
  an earlier scan. Nothing automates this check, by choice, and it is the one
  check whose answer changes while the code sits still — so this is the moment it
  must not be skipped. If it reports standard-library findings, raise the
  `toolchain` directive in `go.mod`, not the `go` directive; the README's
  development block says why.
- Fly's builder must honour that `toolchain` directive. With the default
  `GOTOOLCHAIN=auto` it downloads the pinned version itself; if the build image
  sets `GOTOOLCHAIN=local` the build fails loudly, which is the intended
  behaviour and not something to work around by lowering the pin.

## Comments

2026-08-03: A pre-deployment security review of the app code found no
exploitable vulnerability, but two of its items are **deployment-side** and
belong in this ticket rather than in the code:

- **`ORGANIZER_SECURE` fails open.** `internal/config/config.go` reads
  `os.Getenv("ORGANIZER_SECURE") == "true"` — exact lower-case, defaulting to
  `false`, with no startup warning. Unset, `1`, `TRUE` or a typo all yield a
  session cookie **without** the `Secure` attribute, and since no deployment
  config exists yet, that is the default the first deploy inherits. Two things to
  do: set it in `fly.toml`, and invert the default in code so production is
  secure-by-default with an explicit opt-out for local development (or at minimum
  parse with `strconv.ParseBool` and log loudly when the cookie will not be
  `Secure`). Add HSTS while at it; nothing in the repo sets it today.
  - Noticed in passing: the `Addr`/`DBPath` emptiness checks in `config.go` are
    unreachable — `env()` always substitutes a non-empty fallback. Not this
    ticket's problem, but the file is being touched anyway.
- **CSRF depends on the domain you choose.** There is no CSRF token anywhere; the
  defence is `SameSite=Lax` plus the fact that every mutating route is POST-only
  with no GET-triggered mutations. That holds against cross-site attackers. It
  does **not** isolate the app from a *same-site* host — another host under the
  same registrable domain. On `*.fly.dev` that is a non-issue (it is on the Public
  Suffix List, so sibling apps are cross-site). Under a custom club domain with
  other content on it — a WordPress site, a stale subdomain — it becomes real. So:
  if the app gets `organizer.verein.de` next to anything else, add an
  `Origin`/`Sec-Fetch-Site` check on unsafe methods or switch the cookie to
  `SameSite=Strict` (the app has no cross-site entry flow that would break).

The code-side items from the same review are tracked separately under
`.scratch/pre-deploy-hardening/`.
