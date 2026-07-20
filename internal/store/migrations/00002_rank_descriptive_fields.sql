-- +goose Up

-- Optional, descriptive rank metadata (ADR-0001, issue 03): a major-rank label
-- for grouping/queries (belt colour, e.g. "White") and a numeric sub-level (BJJ
-- stripes, also fits Karate dan). Both are display-only and never participate in
-- a promotion. Empty/zero where a system defines no such dimension. "group" is a
-- reserved word, hence the rank_ prefix on the column.
ALTER TABLE ranks ADD COLUMN rank_group TEXT NOT NULL DEFAULT '';
ALTER TABLE ranks ADD COLUMN degree INTEGER NOT NULL DEFAULT 0;

-- Rank names are unique within a system. This backs the idempotent seed
-- (issue 03) and guards against duplicate ranks generally.
CREATE UNIQUE INDEX ranks_system_name_idx ON ranks (grading_system_id, name);

-- +goose Down
DROP INDEX ranks_system_name_idx;
ALTER TABLE ranks DROP COLUMN degree;
ALTER TABLE ranks DROP COLUMN rank_group;
