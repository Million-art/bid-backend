-- +goose Up
-- +goose StatementBegin
ALTER TABLE bids 
    ADD COLUMN IF NOT EXISTS direct_price DECIMAL(12,2) NOT NULL DEFAULT 0.00;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE bids 
    DROP COLUMN IF EXISTS direct_price;
-- +goose StatementEnd
