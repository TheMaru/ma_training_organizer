# Handling personal data of minors (GDPR)

## Context

The tool stores personal data about children (name, birth date, join date, training-observation notes). Under GDPR this is sensitive, and the club must be able to honour data-subject rights. The club already maintains all member data — including consent — in a central membership system.

## Decision

- **Scope boundary:** this tool is **not** the system of record for membership or consent. Parental consent and membership administration live in the club's central system; processing here is covered by that enrollment. No consent modelling in the app for v1.
- **EU hosting:** the app and its backups are hosted in an EU region (see ADR-0002).
- **Right to erasure:** deleting an athlete is a **hard delete with cascade** (their promotions go too). Backups have a bounded retention (~30 days) so a deletion also disappears from backups within that window.
- **No special-category data (Art. 9):** notes are for training observations relevant to graduation only. A UI hint discourages health/injury or other sensitive data. Deliberate injury/health tracking, if ever needed, would be added later with its own legal basis — not smuggled into a free-text field.
- **Right of access:** satisfied manually in v1 (the data is a handful of visible fields); no export feature. Access requests also run through the central system.
- **Access control:** only authenticated trainers can read any athlete data; transport is HTTPS.

## Consequences

- Data minimisation is baked in: the app deliberately stores little, and nothing in Art. 9.
- Erasure is simple and complete, at the cost of being irreversible (mitigated by short-retention backups).
- A consent flag, data export, and injury tracking are all deferred but easy to add if the club later wants this tool to own them.
