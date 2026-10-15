-- Migration: 00012_lub_platform.sql
-- Creates the three new tables for the Lowest Unique Bid platform.
-- All amounts are stored as DECIMAL(12,2) with exactly 2 decimal places.

-- ============================================================
-- 1. lub_products — auction items
-- ============================================================
CREATE TABLE IF NOT EXISTS lub_products (
    id                         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                       VARCHAR(255) NOT NULL,
    description                TEXT,
    image_url                  TEXT,
    video_url                  TEXT,
    status                     VARCHAR(20)  NOT NULL DEFAULT 'draft',

    -- Pricing
    entrance_fee               DECIMAL(12,2) NOT NULL,
    min_bid_value              DECIMAL(10,2) NOT NULL DEFAULT 0.01,
    max_bid_value              DECIMAL(10,2),

    -- Lifecycle timestamps
    start_at                   TIMESTAMPTZ,
    close_at                   TIMESTAMPTZ,
    executed_at                TIMESTAMPTZ,

    -- Postpone tracking
    postpone_count             INT          NOT NULL DEFAULT 0,
    postponed_at               TIMESTAMPTZ,

    -- Suspension (future use)
    suspended_reason           TEXT,
    suspended_at               TIMESTAMPTZ,
    resume_from_status         VARCHAR(20),

    -- Winner info (set after ExecuteDraw)
    winner_id                  TEXT,
    winner_bid_value           DECIMAL(10,2),
    winner_name                VARCHAR(255),
    winner_email               VARCHAR(255),
    winner_phone               VARCHAR(50),

    -- Financials
    commission_rate            DECIMAL(5,4)  NOT NULL DEFAULT 0.10,
    total_entrance_fee_revenue DECIMAL(12,2) NOT NULL DEFAULT 0,

    rejection_reason           TEXT,

    -- Audit
    created_by                 UUID          NOT NULL REFERENCES users(id),
    created_at                 TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at                 TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_lub_products_status   ON lub_products(status);
CREATE INDEX IF NOT EXISTS idx_lub_products_close_at ON lub_products(close_at);

-- ============================================================
-- 2. lub_participations — entrance fee payment records
-- One row per (product, participant).
-- ============================================================
CREATE TABLE IF NOT EXISTS lub_participations (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id        UUID          NOT NULL REFERENCES lub_products(id) ON DELETE CASCADE,
    participant_id    TEXT          NOT NULL,

    -- Participant snapshot at purchase time
    participant_name  VARCHAR(255),
    participant_email VARCHAR(255),
    participant_phone VARCHAR(50),

    -- Payment
    transaction_id    VARCHAR(255)  UNIQUE,          -- Chapa tx_ref
    fee_paid          DECIMAL(12,2) NOT NULL,

    -- Status: active | refunded
    status            VARCHAR(20)   NOT NULL DEFAULT 'active',

    joined_at         TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    UNIQUE (product_id, participant_id)
);

CREATE INDEX IF NOT EXISTS idx_lub_participations_product     ON lub_participations(product_id);
CREATE INDEX IF NOT EXISTS idx_lub_participations_participant ON lub_participations(participant_id);

-- ============================================================
-- 3. lub_bid_entries — the actual bid values placed
-- A participant may have multiple rows per product (different values).
-- UNIQUE constraint: (product_id, participant_id, bid_value).
-- ============================================================
CREATE TABLE IF NOT EXISTS lub_bid_entries (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id     UUID          NOT NULL REFERENCES lub_products(id) ON DELETE CASCADE,
    participant_id TEXT          NOT NULL,

    bid_value      DECIMAL(10,2) NOT NULL,  -- exactly 2dp, e.g. 1.20

    -- Computed at draw time
    is_unique      BOOLEAN       NOT NULL DEFAULT FALSE,
    is_winner      BOOLEAN       NOT NULL DEFAULT FALSE,

    placed_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    -- Prevent the same participant from placing the same value twice on one product
    UNIQUE (product_id, participant_id, bid_value)
);

CREATE INDEX IF NOT EXISTS idx_lub_bid_entries_product        ON lub_bid_entries(product_id);
CREATE INDEX IF NOT EXISTS idx_lub_bid_entries_participant    ON lub_bid_entries(participant_id);
CREATE INDEX IF NOT EXISTS idx_lub_bid_entries_product_value  ON lub_bid_entries(product_id, bid_value);
