# 04 — Session timeouts, and make them configurable

Status: done

Add an idle timeout, move both session durations into the environment, and make a
misconfiguration visible instead of silent.

Split out of [[pre-deploy-hardening]] 02, which keeps the revocation controls.

## The problem

`SessionLifetime` is 30 days and **hardcoded** in `config.Load`, and no
`IdleTimeout` is set at all, so scs applies none. A captured cookie therefore
stays valid for up to a month regardless of what the account owner does — and
since [[pre-deploy-hardening]] 02 decided that a password change deliberately
does *not* revoke other sessions, the session durations now carry the whole
containment burden between an incident and someone invoking a revocation
control.

`config.go`'s own package comment already claims more than the code delivers:
"Every field is derived from an environment variable so the single binary is
configured entirely by its environment (ADR-0002)". `SessionLifetime` is not.

## Decisions (triage, 2026-08-04)

**7 days idle, inside the existing 30-day absolute lifetime.** The ticket's
original suggestion was 24 hours; it was rejected on usage rhythm — training runs
weekly, typically two or three sessions, so a 24-hour idle timeout means a fresh
login at practically every training, on a phone, at the side of a mat. Seven days
spans "Tuesdays and Thursdays" while still killing an unnoticed stolen cookie
within a week. An idle timeout was preferred over simply shortening the absolute
lifetime because a short absolute window hits the *active* user just as hard as
the inactive one, while only the inactive one carries the risk.

Both numbers are a starting point, and making them configurable is precisely so
they can be revised after a month of real use.

**Both durations become environment variables**, read like the existing three.

**Configuration stays environment-only — the binary does not read `.env`.**
Considered and rejected: loading `.env` in-process (with or without a
dependency). On Fly there is no `.env` — production values come from
`fly.toml [env]` and `fly secrets` — so the loader would be dead code in
production, and a dead production path is where surprises hide. Worse, an
untracked file in the working directory would then reach **tests and every CLI
subcommand**, including which database `seed-demo` or `import-athletes` touches;
that corner has already produced one accident (a mistyped DB variable
demo-seeded the repo's own database). One mechanism everywhere, no precedence
rules to reason about.

**Local ergonomics are solved by a wrapper script, not by the binary.** A small
POSIX `sh` script that sources `.env` when present and `exec`s whatever it is
given, so a child process inherits the environment and the developer's own shell
is never modified:

```sh
#!/bin/sh
set -eu
if [ -f .env ]; then
  set -a
  . ./.env
  set +a
fi
exec "$@"
```

Used as `./dev go run ./cmd/organizer`, and just as importantly
`./dev go run ./cmd/organizer seed-demo` — the CLI subcommands need
`ORGANIZER_DB_PATH` exactly as the server does, and they are the accident-prone
half. The `if` rather than a `&&` one-liner matters: `set -e` would abort on a
missing `.env` otherwise. A `Makefile` was considered and passed over — more
conventional for Go, but it is where a build step eventually grows, and ADR-0002
keeps those out.

**The server logs its resolved configuration at boot** — address, database path,
session lifetime, idle timeout, `Secure`. This is the part that actually
addresses the underlying complaint, because it works even when the wrapper was
bypassed: a wrong database path becomes one visible line instead of silent
correct-looking behaviour. Nothing secret is in `Config`, so the whole struct can
be logged.

## Acceptance

- An idle timeout of 7 days is applied to the session manager, inside the
  unchanged 30-day absolute lifetime.
- Both durations are read from the environment with those values as defaults.
- `.env.example` is committed, listing **every** variable with its default and a
  one-line description. `.env` itself stays gitignored (the entry already
  exists).
- The wrapper script is committed and executable.
- The server logs its resolved configuration once at startup.
- `config.go`'s package comment is true again.
- **The README documents all of it:** the configuration table gains the two new
  variables, and the development section explains `.env.example` → `.env` → the
  wrapper. The table stays the reference; `.env.example` is its copy-pasteable
  form.
- The dead `Addr`/`DBPath` emptiness checks in `config.Load` are unreachable —
  `env()` returns the fallback for an empty value — and should go while the file
  is open. Also noted in issue 07.
- `fly.toml`'s `[env]` block carries the production values when the deployment
  artifacts are written (issue 07) — noted there, not built here.

## Not in scope

- The revocation controls ([[pre-deploy-hardening]] 02).
- `ORGANIZER_SECURE` failing open, which is an issue-07 deployment item.
- Any change to the absolute 30-day lifetime.

## Comments

2026-08-04 — > *This was generated by AI during triage.*

Split out of [[pre-deploy-hardening]] 02 at triage. That ticket had grown to
carry four unrelated pieces of work; this is the half with no UI, no new routes
and no new mechanism — it lives in `config`, the boot path and the README.

The `.env` question was the longest part of the discussion and is recorded above
because the rejected option is the tempting one: reading `.env` in-process is
what most projects do, and the reason not to here is specific to this app's
deployment shape and to its CLI subcommands sharing the same variables.

2026-08-07: Shipped in `9ac7859`. Every acceptance item is in, and the idle
timeout is covered by two behaviour tests in `internal/web/session_test.go` that
drive the manager with a 50ms and a 100ms timeout — one asserts a session dies
after idling, the other that a trainer who keeps clicking is not logged out.
Verified against scs that `Lifetime` still caps absolutely: the deadline is only
reset on `RenewToken`, so seven days idle really does sit inside thirty.

Two deliberate departures.

- **A non-positive duration aborts startup too**, not only an unparseable one.
  The ticket asked for parsing; `0s` or `-24h` would otherwise reach the session
  manager looking plausible and lock trainers out, which is the same failure the
  ticket's "misconfiguration visible instead of silent" line is about. The cost
  is scs's convention that `IdleTimeout = 0` disables the idle timeout — no
  longer reachable through the environment, deliberately, since that is exactly
  the state this ticket exists to remove.
- **The boot log stayed in `serve()`.** The acceptance list says "the server
  logs", and it does. But the reasoning above it argues from the CLI accident —
  a mistyped DB variable demo-seeding the repo's own database — and `seed-demo`
  and `import-athletes` still print nothing about which database they open. One
  `log.Printf` moved from `serve` up into `run` would cover both; it prints a
  session-lifetime and a `secure=` flag that mean nothing to a subcommand, which
  is the only reason not to. Left for the ticket's author to call, and worth
  folding into issue 07 if the answer is yes.

Review found one thing worth acting on beyond that: the idle-timeout rationale
had been restated three times (code, README, `.env.example`), which is the drift
shape `docs/agents/comments.md` names. `internal/config/config.go` now owns the
why; the README and `.env.example` carry defaults and point at it.

`NewSessionManager` grew a fourth parameter rather than taking a
`config.Config`. Two adjacent durations is a real footgun, but passing the
config struct would make `internal/web` depend on `internal/config` to satisfy a
call-site ergonomic, and the tests would have to build configs to start a
server. Left as it is.
