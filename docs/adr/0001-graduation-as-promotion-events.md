# Graduation as promotion events against data-driven grading systems

## Context

The tool tracks martial-arts athletes, and graduation (belts/ranks) is the concept that makes it discipline-agnostic rather than BJJ-specific. Rank systems differ across arts and cohorts — even within BJJ, kids and adults use different systems, and clubs disagree on whether to track stripes/degrees.

## Decision

Model graduation as **promotion events**, not a current-rank field: a `Promotion` records an athlete reaching a `Rank` on a date. The current rank is derived (most recent promotion by date); full history falls out for free.

Ranks are **data-driven, not a hardcoded enum**: a `GradingSystem` is an ordered set of `Rank`s, seeded (e.g. "BJJ Kids", "BJJ Adult"). Rank order is for display only — promotions may skip ranks, and an athlete's history may cross grading systems (kids → adult at 16). An athlete is not bound to a single system.

Stripes/degrees are just additional ranks in a system's list where that system chooses to define them — no dedicated "stripe" concept in the model. This pushes the "how granular are ranks" variability into data rather than code.

To allow grouping and display without splitting the model, a rank may carry **optional, descriptive** metadata: a `group` label (the major rank, e.g. a belt colour) and a numeric `degree` (BJJ stripes, Karate dan). A system fills these where they apply and leaves them empty otherwise. Promotions still target a single rank — these fields never participate in the promotion itself. (Rejected alternative: making belt + stripe a two-part dimension on the promotion; it would require per-rank stripe-limit rules and break discipline-agnosticism.)

## Considered Options

- **Hardcoded BJJ belt enum** — rejected: contradicts the martial-arts-general positioning; can't represent other arts or the kids/adult split cleanly.
- **Current-rank field only** — rejected: loses graduation history, which trainers care about.
- **Rank = belt colour + separate stripe count** — rejected: bakes "stripes" in as a universal sub-dimension, which doesn't fit non-BJJ grade systems.
- **Multiple concurrent grading systems per club** — deferred: a pure data extension of the chosen model, addable later without breaking it.

## Consequences

- "Show current rank" requires a query (latest promotion by date), not a field read — negligible at this scale.
- Seeding grading systems and their ranks is required infrastructure before any promotion can be recorded.
- The awarding trainer is deliberately **not** recorded on a promotion: trainers agree on promotions beforehand, so the attribution carries no value.
