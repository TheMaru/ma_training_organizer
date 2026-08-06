# 05 — Stop trusting client-supplied client-IP headers

Status: ready-for-agent

`internal/web/server.go:45` installs `middleware.RealIP` from
`github.com/go-chi/chi/v5 v5.3.1`. That middleware overwrites `r.RemoteAddr` with
a value taken from the request's own headers: the **leftmost** entry of
`X-Forwarded-For`, or `True-Client-IP`, or `X-Real-IP` — whichever is present,
whether or not the infrastructure in front of the app sets it, and with no notion
of a trusted proxy. Any client can therefore decide what the application believes
its address to be, by sending one header.

chi documents this and has three advisories against it: `GHSA-3fxj-6jh8-hvhx`,
`GHSA-rjr7-jggh-pgcp` and `GHSA-9g5q-2w5x-hmxf`. `staticcheck` reports the
middleware as deprecated for exactly this reason (`SA1019`), which is how it was
found.

## Why this is a fix and not a vulnerability report

Nothing in this application reads `r.RemoteAddr`. The only consumer is
`middleware.Logger`, installed on the next line, which writes it to the request
log. So today the whole impact is **log forgery**: a request can name any address
it likes and the log will record it. There is no authentication decision, no rate
limit and no audit trail keyed on the address, so nothing is bypassable.

It is filed before the first deployment anyway, for two reasons.

The first is that the value looks trustworthy and is not. A future login rate
limiter — the obvious next thing to want, and adjacent to
[[04-session-timeouts-and-configuration]] — would reach for `r.RemoteAddr`
because that is where Go puts the client address, and would silently be
per-attacker-chosen-string rather than per-client. That is the invisible failure
mode: the code would read correctly and not work. Removing the misleading value
now is cheaper than remembering this later.

The second is that the log is the only thing that wanted the real address in the
first place, and it wants it *because* of the deployment (ADR-0002: Fly). Without
the middleware the log shows Fly's proxy address for every request, which is
useless but honest. With it, the log shows whatever the client typed. Neither is
what anyone wants, and the first deployment is the moment to settle which.

## Fix

**Recommended: delete the middleware.** No consumer needs the client address
today, so the honest state is not having it. The request log loses a field that
was never reliable.

**If the client address in the log is genuinely wanted**, it has to come from a
header the proxy sets and *overwrites*, not one it appends to. On Fly that is
`Fly-Client-IP`. That means a small middleware of our own rather than chi's, and
it means the trust boundary gets written down: the app trusts this one header
because the only route to the app is through Fly's proxy, which replaces it. Note
this makes the web layer aware of its host, which ADR-0002 deliberately keeps an
escape hatch from — so it wants either a config value naming the header
(consistent with `ORGANIZER_*`) or a sentence in the ADR log, not a bare literal.

Deciding between the two is part of the ticket. The recommendation is the
deletion, on the grounds that this repo has an established preference for
removing an abstraction until a real need shows up, and the need here is
currently a log line nobody has read.

## Acceptance

- `middleware.RealIP` is gone from `internal/web/server.go`, and
  `go run honnef.co/go/tools/cmd/staticcheck@latest ./...` reports nothing.
- The existing suite passes unchanged. This is a deletion of a middleware no
  assertion depends on, so no new test is written for the deletion itself —
  a test that a log line contains a particular address would be testing chi.
- **Only if the replacement route is taken:** a test that a request carrying
  `X-Forwarded-For`, `X-Real-IP` and `True-Client-IP` does not change what the
  application sees as the client address, and that the trusted header does. That
  test is the trust boundary written as an assertion, and it is the reason the
  replacement is acceptable at all.

## Comments

2026-08-06: Found by adding `staticcheck` to the development commands in the
README, in the same pass that added `govulncheck`. It was the tool's only finding
across the repo — see the README's note on why complexity metrics were tried and
dropped, and this is the counter-example: a dependency behaving differently from
how it reads at the call site is not something reading the code finds.
