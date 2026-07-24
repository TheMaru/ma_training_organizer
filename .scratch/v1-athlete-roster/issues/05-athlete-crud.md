# 05 — Athlete CRUD

Status: done
Blocked by: 04

Manage the shared athlete roster. HTMX-driven forms.

## Scope

- List all athletes, **sortable by last name**.
- Create / edit: `firstName`, `lastName`, `birthDate`, `joinedOn`, `notes`.
- `notes` field shows a UI hint: *"keine Gesundheits-/Sonderdaten"* (ADR-0003).
- Delete: **hard delete**, cascading to the athlete's promotions.
- Shared roster — every trainer sees and edits every athlete (no ownership).

## Acceptance

- Full create / read / update / delete works.
- Deleting an athlete removes their promotions.
- The list sorts by last name.

## Comments

Closed 2026-07-24 — implemented and merged to main (f251506, fix dbb4463).
