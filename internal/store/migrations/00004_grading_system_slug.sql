-- +goose Up

-- A grading system's stable identity outside the database (ADR-0006): the value
-- that travels in the roster's ?system= URL, while name becomes a display label
-- free to be renamed or localized. Hand-authored in the seed, never derived.
-- Defaults to the empty string; the seed fills it on the next boot for rows that
-- predate the column, as ensureGradingSystem already does for sort_order.
ALTER TABLE grading_systems ADD COLUMN slug TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE grading_systems DROP COLUMN slug;
