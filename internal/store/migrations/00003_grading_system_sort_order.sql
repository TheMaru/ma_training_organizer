-- +goose Up

-- Display order for grading systems, data-driven like ranks.sort_order
-- (ADR-0001). Lower sorts first, so the promotion form and any system-grouped
-- view list kids systems ahead of adult ones (progression order). Defaults to 0;
-- the seed (issue 03) assigns the real order and keeps it current on re-seed.
ALTER TABLE grading_systems ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE grading_systems DROP COLUMN sort_order;
