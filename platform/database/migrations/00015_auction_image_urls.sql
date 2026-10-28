-- +goose Up
ALTER TABLE auctions
  ADD COLUMN IF NOT EXISTS image_urls JSONB NOT NULL DEFAULT '[]'::jsonb;

-- Backfill gallery from legacy single cover image when present.
UPDATE auctions
SET image_urls = jsonb_build_array(image_url)
WHERE (image_urls IS NULL OR image_urls = '[]'::jsonb)
  AND image_url IS NOT NULL
  AND btrim(image_url) <> '';

-- +goose Down
ALTER TABLE auctions
  DROP COLUMN IF EXISTS image_urls;
