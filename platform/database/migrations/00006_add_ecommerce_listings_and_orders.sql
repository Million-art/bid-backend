-- +goose Up
-- +goose StatementBegin
ALTER TABLE bids 
    ADD COLUMN IF NOT EXISTS listing_type VARCHAR(20) NOT NULL DEFAULT 'bid',
    ADD COLUMN IF NOT EXISTS stock INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS direct_price DECIMAL(12,2) NOT NULL DEFAULT 0.00;

CREATE INDEX IF NOT EXISTS idx_bids_listing_type ON bids(listing_type);

CREATE TABLE IF NOT EXISTS direct_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    listing_id UUID NOT NULL REFERENCES bids(id) ON DELETE CASCADE,
    buyer_id UUID NOT NULL,
    agent_id UUID NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'pending_payment',
    
    quantity INT NOT NULL DEFAULT 1,
    unit_price DECIMAL(12,2) NOT NULL,
    total_cost DECIMAL(12,2) NOT NULL,
    delivery_fee DECIMAL(10,2) NOT NULL DEFAULT 0.00,
    platform_commission DECIMAL(12,2) NOT NULL DEFAULT 0.00,
    agent_earnings DECIMAL(12,2) NOT NULL DEFAULT 0.00,

    delivery_type VARCHAR(30) NOT NULL DEFAULT 'delivery',
    shipping_address TEXT,
    city VARCHAR(100),
    subcity VARCHAR(100),

    customer_name VARCHAR(255) NOT NULL,
    customer_phone VARCHAR(50) NOT NULL,
    delivery_otp VARCHAR(6),

    dispatched_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_direct_orders_buyer ON direct_orders(buyer_id);
CREATE INDEX IF NOT EXISTS idx_direct_orders_agent ON direct_orders(agent_id);
CREATE INDEX IF NOT EXISTS idx_direct_orders_listing ON direct_orders(listing_id);
CREATE INDEX IF NOT EXISTS idx_direct_orders_status_created ON direct_orders(status, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS direct_orders;
DROP INDEX IF EXISTS idx_bids_listing_type;
ALTER TABLE bids 
    DROP COLUMN IF EXISTS direct_price,
    DROP COLUMN IF EXISTS stock,
    DROP COLUMN IF EXISTS listing_type;
-- +goose StatementEnd
