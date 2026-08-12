# 02 — A session cannot outlive its trainer

Status: ready-for-agent
Blocked by: None — can start immediately
Spec: `.scratch/trainer-offboarding/spec.md`
Parent: `01-deactivate-or-delete-a-trainer.md`

**What to build:** a request whose trainer no longer exists stops being
authenticated. The session is destroyed and the browser lands on the login page,
the same way a request with no session does.

This is a defect that exists today, independent of offboarding: the auth
middleware checks only that a trainer id is present in the session and never
loads the account, so an account removed from the database keeps full access
until its session happens to expire. Nothing surfaces it either — the locale
resolution swallows the not-found error and falls back to `Accept-Language`, so
the pages keep rendering.

It goes first because it is also the prefactor for [[03]]: once the middleware
holds the trainer, refusing a deactivated one is a condition rather than a change
of shape.

- [ ] A logged-in trainer whose account is removed from the database lands on the
      login page on their next request, and the request is not served.
- [ ] The session is destroyed, not merely redirected — a second request behaves
      like a first visit rather than repeating the same check.
- [ ] An active trainer's session is unaffected. A test asserts this, so the new
      check cannot pass by signing everyone out.
- [ ] A second trainer's session is unaffected when the first trainer's account
      goes.
- [ ] The locale resolution no longer needs its silent not-found fallback to be
      load-bearing: whatever guarantees that (middleware ordering) is recorded
      where the code says it, per `docs/agents/comments.md`.
- [ ] Tests live in the `internal/web` black-box seam, driven through the existing
      auth test server, which already returns the database alongside the server
      and client.
- [ ] `go test ./...` passes; the analysers in `docs/agents/analysis.md` run clean.
