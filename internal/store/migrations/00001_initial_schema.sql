-- +goose Up

-- Trainers are the only accounts (CONTEXT.md). Provisioned via CLI, never
-- self-registration, so there is no email/verification column.
CREATE TABLE trainers (
    id            INTEGER PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Session store for alexedwards/scs sqlite3store. The column shape (token/data/
-- expiry) is dictated by that library; do not change it without changing scs.
CREATE TABLE sessions (
    token  TEXT PRIMARY KEY,
    data   BLOB NOT NULL,
    expiry REAL NOT NULL
);
CREATE INDEX sessions_expiry_idx ON sessions (expiry);

-- A grading system is an ordered set of ranks for one discipline-and-cohort,
-- e.g. "BJJ Kids" vs "BJJ Adult" (CONTEXT.md). Seeded in issue 03.
CREATE TABLE grading_systems (
    id   INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

-- A rank is one named position within a grading system. sort_order is for
-- display/sorting only, not a mandatory path (ranks may be skipped).
CREATE TABLE ranks (
    id                INTEGER PRIMARY KEY,
    grading_system_id INTEGER NOT NULL REFERENCES grading_systems (id),
    name              TEXT NOT NULL,
    sort_order        INTEGER NOT NULL
);
CREATE INDEX ranks_grading_system_idx ON ranks (grading_system_id, sort_order);

-- An athlete is a person tracked by trainers. birth_date/joined_on are optional;
-- notes is free text. Hard-deleted with cascade to promotions (spec, ADR-0003).
CREATE TABLE athletes (
    id         INTEGER PRIMARY KEY,
    first_name TEXT NOT NULL,
    last_name  TEXT NOT NULL,
    birth_date DATE,
    joined_on  DATE,
    notes      TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- A promotion is the event of an athlete reaching a rank on a date. Current rank
-- is derived as the most recent promotion by date (issue 06). Deleting the
-- athlete erases their history; ranks are reference data and are not cascaded.
CREATE TABLE promotions (
    id          INTEGER PRIMARY KEY,
    athlete_id  INTEGER NOT NULL REFERENCES athletes (id) ON DELETE CASCADE,
    rank_id     INTEGER NOT NULL REFERENCES ranks (id),
    promoted_on DATE NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX promotions_athlete_idx ON promotions (athlete_id);
CREATE INDEX promotions_rank_idx ON promotions (rank_id);

-- +goose Down
DROP TABLE promotions;
DROP TABLE athletes;
DROP TABLE ranks;
DROP TABLE grading_systems;
DROP TABLE sessions;
DROP TABLE trainers;
