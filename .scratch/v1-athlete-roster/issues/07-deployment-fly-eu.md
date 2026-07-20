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
