# Organizer

A tool for martial-arts trainers to organize the people they train. Discipline-agnostic (not tied to BJJ), unified by the concept of graduation. First capability: a roster of athletes.

## Language

**Athlete**:
A person who trains at the club/school and is tracked by trainers. Not necessarily a child; the tool is martial-arts-general. Distinct from the trainer who logs in to use the tool.
_Avoid_: Member, student, kid, participant

**Roster**:
The complete set of athletes the club tracks — one, total, and shared: every trainer sees and edits all of it, and no athlete belongs to a trainer. Sorting and filtering produce *views* of the roster, never other rosters; a view showing twelve of forty athletes is still a view of the one roster.
_Avoid_: List, table, group, cohort

**Trainer**:
The account of a person who manages athletes and awards promotions. The only kind of account in v1 (one login = one trainer); all trainers are equal, with no in-app roles. Accounts are provisioned out-of-band, not via self-registration, and a trainer stays a Trainer once deactivated.
_Avoid_: Coach, instructor, admin, user

**Deactivated**:
A trainer whose account is kept but refused at login, from a recorded date onwards. Reversible, and distinct from a deleted trainer, whose account is gone entirely.
_Avoid_: Disabled, suspended, archived, inactive, former

**Operator**:
Whoever runs the tool itself rather than using it — provisioning trainers, restoring a password, seeding demo data. Not an account and not a role in the app: a hat, usually worn by one of the trainers, and the reason those acts live on the command line instead of behind a login.
_Avoid_: Admin, sysadmin, maintainer, superuser

**Promotion**:
The event of an athlete reaching a rank on a date. An athlete's current rank is their most recent promotion by date; the full sequence is their graduation history.
_Avoid_: Graduation (the field/state), grading

**GradingSystem**:
An ordered set of ranks for one discipline-and-cohort, e.g. "BJJ Kids" and "BJJ Adult" are two separate systems. Data-driven and seeded, not hardcoded. Its identity is stable and separate from its name, which is a display label and may be renamed or localized.
_Avoid_: Belt system, curriculum, style

**Rank**:
A single named position within a grading system, carrying its order in that system. A promotion targets exactly one rank. Order is for sorting/display only, not a mandatory path — ranks may be skipped. Current rank = the athlete's most recent promotion by date. Its identity is separate from its name, which is a display label and may be localized.
_Avoid_: Belt, grade, level, degree

**Ungraded**:
An athlete with no promotions at all, and therefore with no current rank and no grading system. Distinct from an athlete at the lowest rank, which is a graduation like any other.
_Avoid_: Beginner, white belt, unranked, rank zero
