-- +goose Up
-- +goose StatementBegin

-- 1. Fix check constraints on bids so direct sales (ticket_price=0, total_tickets=0) do not violate constraints
ALTER TABLE bids DROP CONSTRAINT IF EXISTS chk_ticket_price_positive;
ALTER TABLE bids ADD CONSTRAINT chk_ticket_price_positive CHECK (ticket_price >= 0);

ALTER TABLE bids DROP CONSTRAINT IF EXISTS chk_total_tickets_positive;
ALTER TABLE bids ADD CONSTRAINT chk_total_tickets_positive CHECK (total_tickets >= 0);

-- 2. Remove stock column from bids (direct sell is pure classified listing without stock tracking)
ALTER TABLE bids DROP COLUMN IF EXISTS stock;

-- 3. Add telegram fields and joined_via to users
ALTER TABLE users 
    ADD COLUMN IF NOT EXISTS telegram_id BIGINT UNIQUE,
    ADD COLUMN IF NOT EXISTS telegram_username VARCHAR(100),
    ADD COLUMN IF NOT EXISTS joined_via VARCHAR(30) NOT NULL DEFAULT 'web';

CREATE INDEX IF NOT EXISTS idx_users_telegram_id ON users(telegram_id);
CREATE INDEX IF NOT EXISTS idx_users_joined_via ON users(joined_via);

-- 4. Add telegram fields and joined_via to agents
ALTER TABLE agents
    ADD COLUMN IF NOT EXISTS telegram_id BIGINT,
    ADD COLUMN IF NOT EXISTS telegram_username VARCHAR(100),
    ADD COLUMN IF NOT EXISTS joined_via VARCHAR(30) NOT NULL DEFAULT 'web';

CREATE INDEX IF NOT EXISTS idx_agents_telegram_id ON agents(telegram_id);

-- 5. Add telegram fields and joined_via to participant_users
ALTER TABLE participant_users
    ADD COLUMN IF NOT EXISTS telegram_id BIGINT UNIQUE,
    ADD COLUMN IF NOT EXISTS telegram_username VARCHAR(100),
    ADD COLUMN IF NOT EXISTS joined_via VARCHAR(30) NOT NULL DEFAULT 'web';

CREATE INDEX IF NOT EXISTS idx_participant_users_telegram_id ON participant_users(telegram_id);
CREATE INDEX IF NOT EXISTS idx_participant_users_joined_via ON participant_users(joined_via);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE participant_users
    DROP COLUMN IF EXISTS joined_via,
    DROP COLUMN IF EXISTS telegram_username,
    DROP COLUMN IF EXISTS telegram_id;

ALTER TABLE agents
    DROP COLUMN IF EXISTS joined_via,
    DROP COLUMN IF EXISTS telegram_username,
    DROP COLUMN IF EXISTS telegram_id;

ALTER TABLE users
    DROP COLUMN IF EXISTS joined_via,
    DROP COLUMN IF EXISTS telegram_username,
    DROP COLUMN IF EXISTS telegram_id;

ALTER TABLE bids
    ADD COLUMN IF NOT EXISTS stock INT NOT NULL DEFAULT 0;

ALTER TABLE bids DROP CONSTRAINT IF EXISTS chk_ticket_price_positive;
ALTER TABLE bids ADD CONSTRAINT chk_ticket_price_positive CHECK (ticket_price > 0);

ALTER TABLE bids DROP CONSTRAINT IF EXISTS chk_total_tickets_positive;
ALTER TABLE bids ADD CONSTRAINT chk_total_tickets_positive CHECK (total_tickets > 0);
-- +goose StatementEnd
