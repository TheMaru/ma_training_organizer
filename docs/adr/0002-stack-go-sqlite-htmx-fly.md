# Stack: Go + SQLite + server-rendered HTMX on Fly.io

## Context

A self-hosted organization tool for a martial-arts club with a handful of trusted trainers and low traffic. Goals: cheapest possible to run, trivial to put on the internet, minimal ops, and a proper login. The "obvious" path (Next.js + managed Postgres on Vercel) was on the table.

## Decision

- **Backend: Go.** Compiles to a single static binary → the smallest, simplest deployable.
- **Database: SQLite**, a file on a persistent volume next to the app. No separate DB service to run, deploy, or pay for; backup is copying/streaming the file. SQL is kept **portable** (no SQLite-specific features) so a later move to Postgres stays cheap.
- **Frontend: server-rendered HTML (`html/template`) + HTMX.** No build step, no second app, no JSON API boundary. A roster with forms is HTMX's core use case.
- **Auth: server-side sessions (`alexedwards/scs`, SQLite-backed) + `alexedwards/argon2id`.** Sessions are revocable (trainers are granted and revoked access deliberately). Accounts are provisioned via a CLI command, not self-registration; password change is self-service; forgotten passwords are reset via CLI (no email infrastructure).
- **Hosting: Fly.io, EU region**, with a volume for the SQLite file and machine auto-stop on idle. Litestream streams the DB to EU object storage for backups.

## Considered Options

- **Next.js + managed Postgres (Vercel)** — rejected: Vercel's deploy ease is tied to JS/serverless and does not host the DB; for self-hosting a long-running server the advantage inverts, and it adds a Node runtime and more moving parts.
- **Astro / React SPA + Go JSON API** — rejected: adds a build pipeline and an API boundary that a handful of trainers don't need, and breaks the single-artifact deployment.
- **Managed Postgres instead of SQLite** — rejected for v1: a second service and cost with no benefit at this scale; revisit if concurrency, multiple instances, or shared analytics ever arrive.
- **Hetzner VPS instead of Fly.io** — rejected: marginally cheaper compute, but ongoing OS/TLS/backup ops outweigh the saving for a side project.

## Consequences

- Everything ships as one binary + one DB file; deployment is `fly deploy`.
- SQLite pins the app to a single instance with a persistent volume — acceptable, horizontal scaling is never needed here.
- The portable-SQL constraint must be honoured in every query for the Postgres escape hatch to stay open.
