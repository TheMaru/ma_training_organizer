# Static analysis

Which analysis this repo runs, which it deliberately does not, and the evidence
for each. The commands themselves live in the README's development block; this
file is the reasoning behind that list, so nobody has to re-derive it.

This is not an ADR. Choosing which analysers to run is trivially reversible and
so fails this repo's ADR bar (hard-to-reverse *and* surprising *and* carrying a
real trade-off), the same reasoning that gave the deliberate duplication in the
Trainer CRUD functions a code comment instead of an ADR. But there is no function
to hang it on either, so it lives here.

## What runs

**`go vet`** and **`go test`**, as always.

**Two module boundaries are checked by a test**, not by a linter. Both parse every
`.go` file in the module and fail on a package that crossed the line. They are
tests rather than rules in a config file because each is a claim about this
codebase that only this codebase can state — and because `go test ./...` is
already run, so nobody has to remember them.

- `TestSessionIsTheOnlyPackageThatImportsSCS` in `internal/session`: no package
  outside that one imports scs. A claim about **imports**.
- `TestTrainerIsTheOnlyPackageThatActsOnAnAccount` in `internal/trainer`: nothing
  outside that module and `internal/store` calls `store.CreateTrainer`,
  `DeactivateTrainer`, `ReactivateTrainer`, `DeleteTrainer` or
  `CountActiveTrainers`. A claim about **calls**, so it walks the syntax tree
  rather than the import block, and it reads each file's import name so an alias
  does not slip past. What it protects, and why, is in `internal/trainer`'s package
  doc.

**`staticcheck`** — deeper static analysis than `go vet`, notably for a
dependency that behaves differently from how it reads at the call site. It found
one thing across the whole repo: `middleware.RealIP` in `internal/web/server.go`,
fixed by deleting it (`.scratch/pre-deploy-hardening/issues/05`). Clean since;
a non-zero exit is now a regression.

**`govulncheck`** — reports only CVEs the code actually reaches, so its output is
a list to act on rather than to triage.

**`go test -race`** before a deploy. Clean today, and worth knowing why that
proves little: the web suite drives a real server but sequentially, so the session
store and the connection pool under simultaneous logins are unexercised. The race
detector finds nothing if nothing gives it two goroutines. That is a test gap, not
a tooling gap.

## `govulncheck` is the one that needs re-running

Every other check here answers a question about the code, so it only needs asking
when the code changes. `govulncheck` answers a question about the world: new
advisories land against an unchanged repo.

Nothing automates it. That is a decision, not an oversight — no `pre-push` hook
and no CI, on simplicity grounds, with the cost named: no session, no scan. It is
acceptable while there is no production to attack. Issue 07 therefore lists a
fresh scan in its acceptance, because the deploy is the one moment it must not be
skipped, and the automation question is worth reopening once the app is live.

Two things about it are easy to get wrong:

- **Raise the `toolchain` directive in `go.mod`, not the `go` directive.** The
  `go` line is the language version; `govulncheck` reports on the standard
  library of whatever toolchain *builds* the module. Bumping `go` would opt into
  new language semantics for no reason.
- **Do not assume one patch release clears it.** Reaching zero in August 2026 took
  `go1.26.5`: `1.26.1` still left eleven findings and `1.26.2` left six.

## What does not run, and why

Each of these was tried against the repo, not dismissed on principle. The
findings are recorded so the argument does not get had twice.

| Analyser | What it found here |
| --- | --- |
| `gocyclo` | Highest cyclomatic complexity in non-test code: **11**. |
| `errcheck` | 19 findings, every one in a test file, every one `.Body.Close()`. None in production code. |
| `deadcode` | 3 of 4 findings were false positives — it ignores tests, and `internal/store/storetest` exists only for them. |
| `go list -u -m all` | 40 outdated modules, nearly all of them `goose`'s transitive database drivers. |
| An ADR-citation link check | All 9 ADRs are cited from Go code and no citation dangles. |

Two of those deserve a sentence more.

**Complexity metrics are the wrong axis for this codebase.** None of the defects
the `.scratch/store-interface-depth/` work fixed would have scored badly — a call
sequence that must be obeyed has no branches at all, and a missing date validation
has *fewer* branches than a correct one. The metric would have rated the code
perfectly on exactly the day it was worst.

**The outdated-module count is graph noise, not dependency bloat.** The module
graph has 104 entries but the build pulls 86 packages: ClickHouse, Vertica, YDB
and OpenTelemetry arrive through `goose`'s `go.mod` and are never linked into the
binary. `govulncheck` already covers the subset that matters, which is the part
that is both outdated and reachable.

## The general lesson

This project's real defects have been interface shape, a security control weaker
than its own docstring, and missing validation. Static analysis finds none of
those, which is why the list above is short and should stay short. Adding another
linter is not the way to raise quality here.

Two things do work, and both are already in use:

- **A review pass against `CONTEXT.md` and the ADR log.** That is what produced
  the `store-interface-depth` spec, and it is what `/improve-codebase-architecture`
  is for.
- **Fuzzing at security boundaries.** Go's fuzzing is built into the toolchain, so
  it adds no dependency. `FuzzReturnPath` in `internal/web/locale_test.go`
  reproduced the defect reported in `.scratch/pre-deploy-hardening/issues/01` in
  **1.2 seconds**, minimising to the input `"/\t/"` without being told what to
  look for. Note that `go test ./...` runs a fuzz target only against its
  seed corpus and `testdata/fuzz/`; searching for new inputs needs an explicit
  `-fuzz=<Name>`.
