-- +goose Up
-- +goose StatementBegin

-- 1. Create dedicated direct_sales table
CREATE TABLE IF NOT EXISTS direct_sales (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price DECIMAL(12,2) NOT NULL DEFAULT 0.00,
    status VARCHAR(30) NOT NULL DEFAULT 'pending_approval', -- 'draft', 'pending_approval', 'active', 'rejected', 'sold'
    rejection_reason TEXT,
    image_url TEXT,
    video_url TEXT,
    media JSONB,
    category VARCHAR(100),
    location VARCHAR(100),
    approved_by UUID REFERENCES users(id) ON DELETE SET NULL,
    approved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_direct_sales_agent_id ON direct_sales(agent_id);
CREATE INDEX IF NOT EXISTS idx_direct_sales_status ON direct_sales(status);
CREATE INDEX IF NOT EXISTS idx_direct_sales_created_at ON direct_sales(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_direct_sales_status_created ON direct_sales(status, created_at DESC);

-- 2. Drop foreign key constraint on direct_orders if it references bids
ALTER TABLE direct_orders DROP CONSTRAINT IF EXISTS direct_orders_listing_id_fkey;

-- 3. Migrate existing direct_sale records from bids table to direct_sales table
INSERT INTO direct_sales (
    id, agent_id, name, description, price, status, rejection_reason, image_url, video_url, media, approved_at, created_at, updated_at
)
SELECT 
    id, agent_id, name, description, 
    COALESCE(NULLIF(direct_price, 0), ticket_price, 0), 
    status, rejection_reason, image_url, video_url, media, 
    CASE WHEN status = 'active' THEN start_date ELSE NULL END,
    created_at, updated_at
FROM bids 
WHERE listing_type = 'direct_sale'
ON CONFLICT (id) DO NOTHING;

-- 4. Delete migrated direct_sale records from bids table
DELETE FROM bids WHERE listing_type = 'direct_sale';

-- 5. Drop listing_type and direct_price from bids table
ALTER TABLE bids DROP COLUMN IF EXISTS listing_type;
ALTER TABLE bids DROP COLUMN IF EXISTS direct_price;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE bids ADD COLUMN IF NOT EXISTS listing_type VARCHAR(20) NOT NULL DEFAULT 'bid';
ALTER TABLE bids ADD COLUMN IF NOT EXISTS direct_price DECIMAL(12,2) NOT NULL DEFAULT 0.00;

DROP TABLE IF EXISTS direct_sales;

-- +goose StatementEnd

