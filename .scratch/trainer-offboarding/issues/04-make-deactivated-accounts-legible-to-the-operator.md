# 04 — Make deactivated accounts legible to the operator

Status: ready-for-agent
Blocked by: 03
Spec: `.scratch/trainer-offboarding/spec.md`
Parent: `01-deactivate-or-delete-a-trainer.md`

**What to build:** the operator can see which trainers exist and which of them are
deactivated since when, and the two neighbouring subcommands stop being silent
about the state.

The app never shows one trainer to another and has no trainer list at all, so
without this the operator is working blind — including on the last-active-trainer
rule from [[03]], whose refusal is unexplainable if you cannot see who is active.

- [ ] `list-trainers` prints every trainer with their username and state, and for
      a deactivated one the date it happened.
- [ ] The listing carries **no password material**, asserted directly rather than
      implied by a happy-path test.
- [ ] The listing's core returns data and prints nothing; the subcommand wrapper
      renders it — the shape the athlete import already uses, and what makes the
      output assertable without capturing stdout.
- [ ] `list-trainers` takes no username and rejects extra arguments, like the demo
      subcommands.
- [ ] Ordering is stable and obvious (by username), so two runs are comparable.
- [ ] `reset-password` on a deactivated trainer is refused with a message that
      names reactivation, so an operator who meant a homecoming learns the verb.
- [ ] `revoke-sessions` on a deactivated trainer still succeeds — harmless and
      idempotent, and nobody should have to reason about command order during an
      incident.
- [ ] `create-trainer` against a deactivated trainer's username reports that the
      name belongs to a deactivated account, rather than only that it is taken.
      Otherwise the operator hunts for an account the app never displays.
- [ ] Tests use the `cmd/organizer` seam against a migrated database. No
      store-level tests are added.
- [ ] `go test ./...` passes; the analysers in `docs/agents/analysis.md` run clean.
