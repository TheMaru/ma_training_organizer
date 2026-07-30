-- +goose Up

-- The trainer's chosen UI language (ADR-0008). The empty default means "never
-- chose", which the web layer resolves from Accept-Language instead — so rows
-- that predate the column keep the behaviour they had.
ALTER TABLE trainers ADD COLUMN locale TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE trainers DROP COLUMN locale;
