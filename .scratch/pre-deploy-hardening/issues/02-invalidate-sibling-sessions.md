# 02 — Invalidate a trainer's other sessions, and time sessions out

Status: needs-triage

A trainer's session survives everything except that session's own logout. A
password change — self-service (`handleChangePassword`) or CLI
(`reset-password`) — rotates the caller's token and leaves every other session
row in the `sessions` table untouched. `internal/web/auth.go` already concedes
this in a comment. With `SessionLifetime` at 30 days and no idle timeout, a
captured cookie stays valid for up to a month no matter what the account owner
does about it.

## The state of play

The capability is not missing: sessions are server-side rows in SQLite
(ADR-0002's "revocable" is about that, as against stateless JWTs), so an operator
with access to the volume can already `DELETE FROM sessions`. What is missing is
any supported way to do it — no `revoke-sessions`, no `delete-trainer`, and no
automatic invalidation on the one event that obviously implies it.

The pre-deployment security review (2026-08-03) dropped this as a hardening gap
rather than a vulnerability: every path to it presupposes the attacker already
holds a cookie or the device. That does not make it a non-issue — it makes it a
containment question rather than an entry-point question.

## What needs deciding before anyone writes code

1. **Does a password change kill the other sessions?** The intuitive answer is
   yes, and it is what "I think someone has my session" pushes a person to do. But
   the current behaviour is deliberate enough to be written down in a comment, so
   the reversal should be a decision, not a drive-by. `scs` exposes
   `SessionManager.Iterate`, so walking the store and destroying every session
   whose `trainerID` matches (skipping the current token on the self-service path)
   is a dozen lines.
2. **Is there an idle timeout, and how long?** A 30-day absolute lifetime with no
   idle timeout is generous for a tool holding personal data of minors (ADR-0003).
   But trainers use this from a phone at the side of a mat; a 30-minute timeout
   would be actively hostile. Something like 24 hours idle inside the 30-day
   absolute lifetime is the shape to argue about.
3. **Is offboarding a feature?** There is no `delete-trainer` and no way to
   deactivate an account — a trainer who leaves the club keeps working
   credentials. That is a real gap in a tool whose accounts are provisioned
   out-of-band, but it is a feature question (what happens to the athletes they
   entered? is deactivation different from deletion?) rather than a fix, and it
   may deserve its own ticket rather than riding along here.

## Acceptance

Once triaged, whatever is decided needs a test that a second session goes dead:
log in with two clients, change the password on one, assert the other's next
request lands on `/login`. `newAuthTestServer` plus a second `newClient` already
makes that a short test — `TestLanguageChoicePersistsOnTheAccount` does the
two-client dance already.

## Comments

2026-08-03: Filed from the pre-deployment security review. Filed as
`needs-triage` because all three questions above are policy calls with real UX
cost, not implementation details.
