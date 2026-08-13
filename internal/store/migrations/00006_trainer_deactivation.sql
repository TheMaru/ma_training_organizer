-- +goose Up

-- The date a trainer's account stopped being able to log in (ADR-0010). NULL
-- means active, so every row that predates the column is active — and the state
-- carries its own date, which a boolean could not.
ALTER TABLE trainers ADD COLUMN deactivated_at TIMESTAMP;

-- +goose Down
ALTER TABLE trainers DROP COLUMN deactivated_at;
