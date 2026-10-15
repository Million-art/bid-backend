-- +goose Up
-- +goose StatementBegin
ALTER TABLE bids DROP CONSTRAINT IF EXISTS chk_bid_status_valid;

ALTER TABLE bids
    ADD CONSTRAINT chk_bid_status_valid CHECK (
        status IN (
            'draft',
            'pending_approval',
            'approved',
            'coming_soon',
            'active',
            'locked',
            'suspended',
            'executed',
            'cancelled',
            'rejected'
        )
    );
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE bids DROP CONSTRAINT IF EXISTS chk_bid_status_valid;

ALTER TABLE bids
    ADD CONSTRAINT chk_bid_status_valid CHECK (
        status IN (
            'draft',
            'pending_approval',
            'approved',
            'active',
            'locked',
            'suspended',
            'executed',
            'cancelled',
            'rejected'
        )
    );
-- +goose StatementEnd
