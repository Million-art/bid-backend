-- +goose Up
-- +goose StatementBegin

-- 1. Add phone column to agents table for direct seller contact
ALTER TABLE agents ADD COLUMN IF NOT EXISTS phone VARCHAR(50);
CREATE INDEX IF NOT EXISTS idx_agents_phone ON agents(phone);

-- 2. Backfill existing agent phones from users table
UPDATE agents 
SET phone = users.phone 
FROM users 
WHERE agents.user_id = users.id 
  AND users.phone IS NOT NULL 
  AND users.phone != '' 
  AND (agents.phone IS NULL OR agents.phone = '');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_agents_phone;
ALTER TABLE agents DROP COLUMN IF EXISTS phone;
-- +goose StatementEnd

