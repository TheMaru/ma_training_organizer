# 02 — A session cannot outlive its trainer

Status: done
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

- [x] A logged-in trainer whose account is removed from the database lands on the
      login page on their next request, and the request is not served.
- [x] The session is destroyed, not merely redirected — a second request behaves
      like a first visit rather than repeating the same check.
- [x] An active trainer's session is unaffected. A test asserts this, so the new
      check cannot pass by signing everyone out.
- [x] A second trainer's session is unaffected when the first trainer's account
      goes.
- [~] The locale resolution no longer needs its silent not-found fallback to be
      load-bearing: whatever guarantees that (middleware ordering) is recorded
      where the code says it, per `docs/agents/comments.md`. **Recorded, but the
      premise was wrong — see the comment below.**
- [x] Tests live in the `internal/web` black-box seam, driven through the existing
      auth test server, which already returns the database alongside the server
      and client.
- [x] `go test ./...` passes; the analysers in `docs/agents/analysis.md` run clean.

## Comments

**2026-08-12 — shipped in `a31a365`.**

`requireAuth` now loads the session's trainer on every request and, on
`ErrTrainerNotFound`, destroys the session and redirects to `/login`. Two tests in
`internal/web/session_test.go` cover it: the account-is-gone case (which asserts
the client's cookie jar is emptied, that being the observable difference between a
destroyed session and a merely refused one) and the colleague-survives case. Both
were checked by mutation — deleting the `Destroy` call, and refusing every session
— so neither passes by construction.

**The fifth acceptance line rested on a wrong premise, so it is only half met.**
It assumed the middleware ordering would make the locale fallback dead code once
`requireAuth` decided first. It does not: `resolveLocale` is registered *outside*
`requireAuth` in `Handler`, because the login page needs a locale too, so it still
runs first and still sees a session naming a deleted account. The fallback also
stays genuinely load-bearing for the login page — a `POST /login` carrying a stale
cookie renders in exactly that fallback locale. What is recorded at `localeFor` is
therefore the narrower true claim, not the ticket's: catching a vanished account
is `requireAuth`'s job, not that fallback's.

Making the ticket's version true would mean running `requireAuth` before
`resolveLocale` and having the middleware put the trainer in the request context —
which would also collapse the two `TrainerByID` queries an authenticated request
now makes into one. That is a change to the middleware chain's shape rather than
this ticket's defect fix, so it was not done here. **Open for a decision**; the
spec's "the read is effectively free" claim is wrong either way, and the deploy
ticket may want it.
