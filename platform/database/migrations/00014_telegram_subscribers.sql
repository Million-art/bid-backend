-- +goose Up
CREATE TABLE IF NOT EXISTS telegram_subscribers (
  telegram_id BIGINT PRIMARY KEY,
  telegram_username VARCHAR(100),
  full_name VARCHAR(255),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_telegram_subscribers_username ON telegram_subscribers (telegram_username);

-- +goose Down
DROP TABLE IF EXISTS telegram_subscribers;
