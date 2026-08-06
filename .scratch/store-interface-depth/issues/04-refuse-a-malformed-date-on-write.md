# 04 — Refuse a malformed date on write

**What to build:** A malformed date is refused instead of stored. This is the one
ticket in the set with a real defect behind it, reproduced during grilling:

```
CreateAthlete with BirthDate "morgen"  ->  accepted, written
ListAthletes                           ->  scan error: string into *time.Time
ListRoster                             ->  scan error: same place
```

So a single POST to the athlete form with a malformed birth date takes the
**shared Roster page** down with a server error for **every** Trainer, until
somebody edits the database by hand. The athlete form performs no date validation
at all — the browser's date input is the only guard, and it is client-side. The
promotion form does guard its date, but only there; the store accepts whatever it
is handed.

Both halves are needed and they do different jobs. The store refusing the value
on write is the backstop that closes the seam — it holds for the CLI, the CSV
import and any client that skips the form. The form checking it is what turns the
refusal into a message the Trainer can act on, with the entered values preserved,
rather than a server error. The promotion form already demonstrates the shape to
follow.

A dedicated date type that makes a malformed value unconstructable was considered
and deferred, with reasons in the spec. It is the stronger form; it is not a net
reduction in code and it ripples much further.

Spec: [[spec]].

**Blocked by:** [[02-open-a-database-in-one-call]] — the new tests should use the
shared fixture rather than be rewritten after it lands.

**Status:** done

- [x] Saving an Athlete with a malformed birth date is refused with a message, the entered values preserved, and a client-error status — not a server error
- [x] The same holds for the joined-on date
- [x] Blank dates remain valid and are still stored as absent, for both fields
- [x] The store refuses a malformed date on athlete create, athlete update and promotion create, with a distinct error a caller can match on
- [x] A malformed promotion date is refused even when the form is bypassed
- [x] Regression test: after an attempted write with a malformed date, the Roster listing and the plain athlete listing both run cleanly. This is the reproduction above, pinned
- [x] The ISO layout is spelled once and shared — the CSV import stops carrying its own copy, while keeping its German date input format
- [x] Coverage recorded before and after as absolute statement counts

## Comments

Shipped 2026-08-06 in `a0115a4`.

**How it landed.** `internal/store/date.go` is the new home of the invariant:
`ISODate` (now exported, the layout's only spelling), the `ErrMalformedDate`
sentinel, and the two checks behind it — `checkDate` for a required date,
`checkOptionalDate` for a nullable one, where blank is absent rather than
malformed. `insertAthlete` carries the athlete check, so the single-row and the
bulk CSV path get it from one place; `UpdateAthlete` and `CreatePromotion` call it
themselves. The athlete form gained the date check the promotion form already had,
and both now parse against `store.ISODate` instead of their own literal.

Blank is malformed for a promotion, which the ticket did not spell out: the column
is NOT NULL and `CurrentRank` compares those dates as strings, so there is no
absent promotion date. The form already rejected it; now the store does too.

**The reproduction, pinned twice.** At the store seam in
`TestARefusedDateLeavesEveryListingReadable`, and through HTTP in
`TestCreateAthleteRefusesAMalformedDate`, which also asserts the roster page still
answers 200 afterwards. Before the fix the HTTP test failed with exactly the error
from the ticket — `storing driver.Value type string into type *time.Time` — on
both dates.

**Coverage, before and after.** Before: **799 of 1087** statements. After: **820
of 1108** — 21 new statements, all of them covered.
