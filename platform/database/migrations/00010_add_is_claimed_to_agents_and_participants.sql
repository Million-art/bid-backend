-- +goose Up
-- +goose StatementBegin

-- 1. Add is_claimed column to agents table for proxy pre-registration
ALTER TABLE agents ADD COLUMN IF NOT EXISTS is_claimed BOOLEAN NOT NULL DEFAULT TRUE;

-- 2. Add is_claimed column to participant_users table
ALTER TABLE participant_users ADD COLUMN IF NOT EXISTS is_claimed BOOLEAN NOT NULL DEFAULT TRUE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE agents DROP COLUMN IF EXISTS is_claimed;
ALTER TABLE participant_users DROP COLUMN IF EXISTS is_claimed;
-- +goose StatementEnd
