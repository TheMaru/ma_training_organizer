# 06 — Promotions & current rank

Status: ready-for-agent
Blocked by: 03, 05

Record graduation as promotion events and derive the current rank (ADR-0001).

## Scope

- On the athlete detail view: record a `Promotion` by choosing grading system → rank → date.
- List the athlete's full promotion history.
- Show the **derived current rank** = most recent promotion by date, **cross-system** (kids → adult).
- Ranks may be skipped (no contiguity requirement).
- No awarding-trainer field.

## Acceptance

- Recording a promotion updates the shown current rank.
- Skipping ranks works (e.g. white → white-2-stripes directly).
- Switching systems (kids → adult) yields the adult rank as current when its date is latest.
