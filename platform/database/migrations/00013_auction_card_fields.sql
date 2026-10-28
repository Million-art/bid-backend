-- +goose Up
ALTER TABLE auctions
  ADD COLUMN IF NOT EXISTS retail_value DECIMAL(12,2) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS min_participants INTEGER NOT NULL DEFAULT 10;

-- +goose Down
ALTER TABLE auctions
  DROP COLUMN IF EXISTS retail_value,
  DROP COLUMN IF EXISTS min_participants;
