-- +goose Up
DROP TABLE IF EXISTS telegram_subscribers;

-- +goose Down
SELECT 1;
