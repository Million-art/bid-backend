-- +goose Up
-- +goose StatementBegin
-- Consolidated initial schema (squashed from migrations 00001–00011).
-- Platform commission on ticket revenue: commission_rate default 0.10 (10%).

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    full_name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    phone VARCHAR(255),
    role VARCHAR(20) NOT NULL,
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);

CREATE TABLE IF NOT EXISTS agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
    display_name VARCHAR(255) NOT NULL,
    kyc_status VARCHAR(20) DEFAULT 'pending',
    didit_session_id VARCHAR(255),
    didit_url VARCHAR(255),
    country VARCHAR(255),
    city VARCHAR(255),
    street VARCHAR(255),
    house_no VARCHAR(255),
    location VARCHAR(255),
    rejection_reason VARCHAR(255),
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    kyc_attempts BIGINT DEFAULT 0,
    last_kyc_attempt_at TIMESTAMPTZ
    -- NOTE: bank_account column removed (use agent_bank_accounts table)
);

CREATE TABLE IF NOT EXISTS participant_users (
    id UUID PRIMARY KEY,
    email VARCHAR(255),
    full_name VARCHAR(255) NOT NULL,
    phone VARCHAR(50),
    available_balance DECIMAL(12,2) NOT NULL DEFAULT 0,
    pending_balance DECIMAL(12,2) NOT NULL DEFAULT 0,
    location VARCHAR(255),
    password_hash VARCHAR(255),
    auth_provider VARCHAR(50) NOT NULL DEFAULT 'password',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    CONSTRAINT chk_participant_available_balance_non_negative CHECK (available_balance >= 0),
    CONSTRAINT chk_participant_pending_balance_non_negative CHECK (pending_balance >= 0)
);
CREATE INDEX IF NOT EXISTS idx_participant_users_deleted_at ON participant_users(deleted_at);
CREATE INDEX IF NOT EXISTS idx_participant_email ON participant_users(email);
CREATE INDEX IF NOT EXISTS idx_participant_phone ON participant_users(phone);

CREATE TABLE IF NOT EXISTS bids (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(20) DEFAULT 'draft',
    total_tickets BIGINT NOT NULL,
    ticket_price DECIMAL(10,2) NOT NULL,
    start_date TIMESTAMPTZ,
    rejection_reason VARCHAR(255),
    
    -- Staff Suspension
    suspended_reason TEXT,
    resume_from_status VARCHAR(20),
    suspended_at TIMESTAMPTZ,

    -- Draw Info
    winner_id UUID,
    winner_ticket_number INT,
    executed_at TIMESTAMPTZ,
    scheduled_at TIMESTAMPTZ,
    expected_draw_at TIMESTAMPTZ,
    draw_postpone_count BIGINT DEFAULT 0,
    draw_replay_video_url TEXT,
    
    -- Refund Window
    refund_window_started_at TIMESTAMPTZ,
    refund_window_ends_at TIMESTAMPTZ,

    -- Inflation/Protection & Limits
    price_update_locked BOOLEAN DEFAULT FALSE,
    max_tickets_per_user BIGINT DEFAULT 0,

    -- Referral Bounty
    accepts_referral_rewards BOOLEAN DEFAULT FALSE,
    referral_bounty_pct DECIMAL(5,2) DEFAULT 0,

    media JSONB,
    image_url VARCHAR(255),
    video_url VARCHAR(255),
    tickets_sold BIGINT DEFAULT 0,
    commission_rate DECIMAL(5,4) DEFAULT 0.10, -- platform fee (10% of ticket revenue)
    total_revenue DECIMAL(12,2) DEFAULT 0,
    platform_commission DECIMAL(12,2) DEFAULT 0,
    agent_earnings DECIMAL(12,2) DEFAULT 0,
    payout_status VARCHAR(30) DEFAULT 'pending',
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    winner_name VARCHAR(255),
    winner_email VARCHAR(255),
    winner_phone VARCHAR(50),
    CONSTRAINT chk_bid_status_valid CHECK (status IN (
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
    )),
    CONSTRAINT chk_bid_payout_status_valid CHECK (payout_status IN (
        'pending',
        'winner_confirmed',
        'ready_for_payout',
        'paid',
        'winner_compensated'
    )),
    CONSTRAINT chk_ticket_price_positive CHECK (ticket_price > 0),
    CONSTRAINT chk_total_tickets_positive CHECK (total_tickets > 0),
    CONSTRAINT chk_referral_bounty_valid CHECK (referral_bounty_pct BETWEEN 0 AND 100)
);
CREATE INDEX IF NOT EXISTS idx_bids_agent_id ON bids(agent_id);
CREATE INDEX IF NOT EXISTS idx_bids_status ON bids(status);
CREATE INDEX IF NOT EXISTS idx_bids_winner_id ON bids(winner_id);
CREATE INDEX IF NOT EXISTS idx_bids_deleted_at ON bids(deleted_at);
CREATE INDEX IF NOT EXISTS idx_bids_refund_window_ends_at ON bids(refund_window_ends_at);

CREATE TABLE IF NOT EXISTS tickets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bid_id UUID NOT NULL REFERENCES bids(id) ON DELETE CASCADE,
    participant_id UUID NOT NULL,
    participant_name VARCHAR(255),
    participant_email VARCHAR(255),
    participant_phone VARCHAR(50),
    quantity BIGINT NOT NULL,
    purchase_time TIMESTAMPTZ,
    transaction_id VARCHAR(255),
    refund_choice VARCHAR(20) DEFAULT 'pending',
    price_paid DECIMAL(10,2) NOT NULL DEFAULT 0,
    status VARCHAR(20) DEFAULT 'active',
    is_referral_award BOOLEAN NOT NULL DEFAULT false,
    referred_by UUID,
    ticket_numbers JSONB,
    CONSTRAINT chk_ticket_status_valid CHECK (status IN (
        'active', 
        'adjustment_required', 
        'refunded'
    )),
    CONSTRAINT chk_ticket_refund_choice_valid CHECK (refund_choice IN (
        'pending', 
        'continue', 
        'withdraw'
    )),
    CONSTRAINT chk_quantity_positive CHECK (quantity > 0)
);
CREATE INDEX IF NOT EXISTS idx_tickets_bid_id ON tickets(bid_id);
CREATE INDEX IF NOT EXISTS idx_tickets_participant_id ON tickets(participant_id);
CREATE INDEX IF NOT EXISTS idx_tickets_transaction_id ON tickets(transaction_id);
CREATE INDEX IF NOT EXISTS idx_tickets_refund_choice ON tickets(refund_choice);
CREATE INDEX IF NOT EXISTS idx_tickets_status ON tickets(status);
CREATE INDEX IF NOT EXISTS idx_tickets_referred_by ON tickets(referred_by);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tickets_transaction_id_unique
ON tickets (transaction_id)
WHERE transaction_id IS NOT NULL
  AND transaction_id <> ''
  AND transaction_id <> 'client';

-- Normalized ticket slot allocation (one row per seat number)
CREATE TABLE IF NOT EXISTS bid_ticket_slots (
    bid_id UUID NOT NULL REFERENCES bids(id) ON DELETE CASCADE,
    ticket_number INT NOT NULL,
    ticket_id UUID NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    PRIMARY KEY (bid_id, ticket_number)
);
CREATE INDEX IF NOT EXISTS idx_bid_ticket_slots_ticket_id ON bid_ticket_slots(ticket_id);

-- Chapa payment ledger (client-backend fulfillment / retries)
CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tx_ref VARCHAR(255) NOT NULL,
    chapa_reference VARCHAR(255),
    user_id UUID,
    bid_id UUID,
    quantity INT NOT NULL DEFAULT 1,
    amount DECIMAL(12,2) NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    processed BOOLEAN NOT NULL DEFAULT false,
    retry_disabled BOOLEAN NOT NULL DEFAULT FALSE,
    referred_by UUID,
    ticket_numbers JSONB,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT payments_tx_ref_unique UNIQUE (tx_ref)
);
CREATE INDEX IF NOT EXISTS idx_payments_success_unprocessed
ON payments (updated_at)
WHERE status = 'success' AND processed = false;
CREATE INDEX IF NOT EXISTS idx_payments_retry_eligible
    ON payments (updated_at)
    WHERE status = 'success' AND processed = FALSE AND retry_disabled = FALSE;
CREATE INDEX IF NOT EXISTS idx_payments_user_id ON payments(user_id);
CREATE INDEX IF NOT EXISTS idx_payments_bid_id ON payments(bid_id);
CREATE INDEX IF NOT EXISTS idx_payments_chapa_reference ON payments(chapa_reference)
    WHERE chapa_reference IS NOT NULL AND chapa_reference <> '';

CREATE TABLE IF NOT EXISTS participations (
    bid_id UUID NOT NULL REFERENCES bids(id) ON DELETE CASCADE,
    participant_id UUID NOT NULL,
    ticket_count BIGINT NOT NULL DEFAULT 0,
    first_joined_at TIMESTAMPTZ NOT NULL,
    last_joined_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (bid_id, participant_id)
);
CREATE INDEX IF NOT EXISTS idx_participations_participant_id ON participations(participant_id);
CREATE INDEX IF NOT EXISTS idx_participations_last_joined_at ON participations(last_joined_at);

CREATE TABLE IF NOT EXISTS platform_wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    balance DECIMAL(12,2) NOT NULL DEFAULT 0,
    pending_balance DECIMAL(12,2) NOT NULL DEFAULT 0,
    total_revenue DECIMAL(12,2) NOT NULL DEFAULT 0,
    currency VARCHAR(3) DEFAULT 'ETB',
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    CONSTRAINT chk_platform_balance_non_negative CHECK (balance >= 0),
    CONSTRAINT chk_platform_pending_balance_non_negative CHECK (pending_balance >= 0)
);

CREATE TABLE IF NOT EXISTS agent_wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL UNIQUE REFERENCES agents(id),
    pending_balance DECIMAL(12,2) DEFAULT 0,
    total_earned DECIMAL(12,2) DEFAULT 0,
    withdrawal_reserved DECIMAL(12,2) DEFAULT 0,
    total_paid DECIMAL(12,2) DEFAULT 0,
    currency VARCHAR(3) DEFAULT 'ETB',
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    CONSTRAINT chk_agent_pending_balance_non_negative CHECK (pending_balance >= 0),
    CONSTRAINT chk_agent_total_earned_non_negative CHECK (total_earned >= 0),
    CONSTRAINT chk_agent_withdrawal_reserved_non_negative CHECK (withdrawal_reserved >= 0),
    CONSTRAINT chk_agent_total_paid_non_negative CHECK (total_paid >= 0)
);

CREATE TABLE IF NOT EXISTS wallet_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_type VARCHAR(20) NOT NULL,
    wallet_id UUID NOT NULL,
    bid_id UUID,
    transaction_type VARCHAR(50) NOT NULL,
    amount DECIMAL(12,2) NOT NULL,
    balance_before DECIMAL(12,2),
    balance_after DECIMAL(12,2),
    description TEXT,
    metadata JSONB,
    created_by UUID,
    created_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS winner_confirmations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bid_id UUID NOT NULL UNIQUE REFERENCES bids(id),
    winner_id UUID NOT NULL,
    winner_name VARCHAR(255),
    winner_email VARCHAR(255),
    winner_phone VARCHAR(50),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    confirmed_at TIMESTAMPTZ,
    confirmation_method VARCHAR(50),
    prize_received BOOLEAN,
    prize_matches_description BOOLEAN,
    rating BIGINT,
    feedback TEXT,
    not_delivered_reason TEXT,
    verified_by UUID,
    verified_at TIMESTAMPTZ,
    reminder_sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS agent_bank_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id),
    bank_name VARCHAR(100) NOT NULL,
    encrypted_account_number TEXT NOT NULL,
    account_last4 VARCHAR(4) NOT NULL DEFAULT '0000',
    account_holder_name VARCHAR(255) NOT NULL,
    is_verified BOOLEAN DEFAULT FALSE,
    is_primary BOOLEAN DEFAULT FALSE,
    verification_document_url TEXT,
    verified_by UUID,
    verified_at TIMESTAMPTZ,
    edited_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS payout_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bid_id UUID REFERENCES bids(id),
    agent_id UUID NOT NULL REFERENCES agents(id),
    amount DECIMAL(12,2) NOT NULL,
    bank_account_id UUID,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    initiated_by UUID,
    initiated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_by UUID,
    completed_at TIMESTAMPTZ,
    transaction_ref VARCHAR(255),
    payment_method VARCHAR(50),
    notes TEXT,
    metadata JSONB,
    CONSTRAINT chk_payout_status_valid CHECK (status IN (
        'pending',
        'processing',
        'completed',
        'failed',
        'rejected'
    ))
);

CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    user_type VARCHAR(20) NOT NULL,
    type VARCHAR(50) NOT NULL,
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    action_url TEXT,
    action_label VARCHAR(50),
    bid_id UUID,
    confirmation_id UUID,
    payout_id UUID,
    is_read BOOLEAN DEFAULT FALSE,
    read_at TIMESTAMPTZ,
    priority VARCHAR(20) DEFAULT 'normal',
    metadata JSONB,
    created_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_deleted_at ON notifications(deleted_at);

CREATE TABLE IF NOT EXISTS notification_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    user_type VARCHAR(20) NOT NULL,
    email_enabled BOOLEAN DEFAULT TRUE,
    in_app_enabled BOOLEAN DEFAULT TRUE,
    push_enabled BOOLEAN DEFAULT FALSE,
    winner_confirmation BOOLEAN DEFAULT TRUE,
    payout_updates BOOLEAN DEFAULT TRUE,
    bid_updates BOOLEAN DEFAULT TRUE,
    marketing BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_notification_prefs_deleted_at ON notification_preferences(deleted_at);

CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    actor_role VARCHAR(32),
    actor_email VARCHAR(255),
    action VARCHAR(255) NOT NULL,
    entity VARCHAR(255) NOT NULL,
    entity_id VARCHAR(255) NOT NULL,
    old_value TEXT,
    new_value TEXT,
    ip_address VARCHAR(255),
    user_agent VARCHAR(255),
    metadata JSONB,
    created_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_deleted_at ON audit_logs(deleted_at);

CREATE TABLE IF NOT EXISTS platform_legal_documents (
    doc_type VARCHAR(32) PRIMARY KEY,
    content_html TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT chk_legal_doc_type CHECK (doc_type IN ('privacy', 'terms'))
);

CREATE TABLE IF NOT EXISTS refund_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bid_id UUID NOT NULL REFERENCES bids(id),
    participant_id UUID NOT NULL REFERENCES participant_users(id),
    ticket_id UUID REFERENCES tickets(id),
    amount DECIMAL(12, 2) NOT NULL,
    tx_ref VARCHAR(255) NOT NULL,
    chapa_refund_ref VARCHAR(255),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    error_message TEXT,
    processed_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT chk_refund_status_valid CHECK (status IN (
        'pending',
        'processing',
        'success',
        'failed',
        'reversed'
    ))
);
CREATE INDEX IF NOT EXISTS idx_refund_records_bid_id ON refund_records(bid_id);
CREATE INDEX IF NOT EXISTS idx_refund_records_ticket_id ON refund_records(ticket_id);
CREATE INDEX IF NOT EXISTS idx_refund_records_status ON refund_records(status);
CREATE UNIQUE INDEX IF NOT EXISTS ux_refund_records_ticket_id
    ON refund_records(ticket_id) WHERE ticket_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_refund_records_tx_ref ON refund_records(tx_ref);

CREATE TABLE IF NOT EXISTS featured_pricings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    placement VARCHAR(50) NOT NULL UNIQUE,
    price_per_day DECIMAL(12,2) NOT NULL DEFAULT 0,
    currency VARCHAR(3) NOT NULL DEFAULT 'ETB',
    updated_by UUID REFERENCES users(id),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW()
);
INSERT INTO featured_pricings (placement, price_per_day, currency)
VALUES ('home_carousel', 0, 'ETB')
ON CONFLICT (placement) DO NOTHING;

CREATE TABLE IF NOT EXISTS featured_campaigns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bid_id UUID NOT NULL REFERENCES bids(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    placement VARCHAR(50) NOT NULL DEFAULT 'home_carousel',
    status VARCHAR(20) NOT NULL DEFAULT 'pending_payment',
    starts_at TIMESTAMPTZ,
    ends_at TIMESTAMPTZ,
    days INT NOT NULL DEFAULT 1,
    amount_paid DECIMAL(12,2) NOT NULL DEFAULT 0,
    currency VARCHAR(3) NOT NULL DEFAULT 'ETB',
    chapa_tx_ref VARCHAR(64) UNIQUE,
    chapa_trx_ref VARCHAR(64),
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_featured_campaigns_bid_id ON featured_campaigns(bid_id);
CREATE INDEX IF NOT EXISTS idx_featured_campaigns_agent_id ON featured_campaigns(agent_id);
CREATE INDEX IF NOT EXISTS idx_featured_campaigns_status ON featured_campaigns(status);
CREATE INDEX IF NOT EXISTS idx_featured_campaigns_placement ON featured_campaigns(placement);

CREATE TABLE IF NOT EXISTS bid_attendance (
    bid_id UUID NOT NULL REFERENCES bids(id) ON DELETE CASCADE,
    participant_id UUID NOT NULL REFERENCES participant_users(id) ON DELETE CASCADE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bid_id, participant_id)
);
CREATE INDEX IF NOT EXISTS idx_bid_attendance_bid_last_seen ON bid_attendance(bid_id, last_seen_at DESC);

-- Performance Indexes (Merged from 00002)
CREATE INDEX IF NOT EXISTS idx_tickets_participant_bid ON tickets(participant_id, bid_id);
CREATE INDEX IF NOT EXISTS idx_notifications_unread_count ON notifications(user_id, user_type, is_read) WHERE deleted_at IS NULL AND is_read = false;
CREATE INDEX IF NOT EXISTS idx_featured_campaigns_active_placement_dates ON featured_campaigns(placement, starts_at, ends_at) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payout_records_agent_id ON payout_records(agent_id);
CREATE INDEX IF NOT EXISTS idx_payout_records_status ON payout_records(status);
CREATE INDEX IF NOT EXISTS idx_agent_bank_accounts_agent_id ON agent_bank_accounts(agent_id);
CREATE INDEX IF NOT EXISTS idx_wallet_transactions_wallet_id ON wallet_transactions(wallet_id);
CREATE INDEX IF NOT EXISTS idx_refund_records_participant_id ON refund_records(participant_id);
CREATE INDEX IF NOT EXISTS idx_bids_auto_reject ON bids (expected_draw_at, draw_postpone_count, status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_winner_confirmations_pending ON winner_confirmations (status, created_at) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_tickets_participation_sync ON tickets (bid_id, participant_id, status) WHERE status = 'active';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS bid_attendance;
DROP TABLE IF EXISTS featured_campaigns;
DROP TABLE IF EXISTS featured_pricings;
DROP TABLE IF EXISTS refund_records;
DROP TABLE IF EXISTS platform_legal_documents;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS notification_preferences;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS payout_records;
DROP TABLE IF EXISTS agent_bank_accounts;
DROP TABLE IF EXISTS winner_confirmations;
DROP TABLE IF EXISTS wallet_transactions;
DROP TABLE IF EXISTS agent_wallets;
DROP TABLE IF EXISTS platform_wallets;
DROP TABLE IF EXISTS participations;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS bid_ticket_slots;
DROP TABLE IF EXISTS tickets;
DROP TABLE IF EXISTS bids;
DROP TABLE IF EXISTS participant_users;
DROP TABLE IF EXISTS agents;
DROP TABLE IF EXISTS users;

DROP INDEX IF EXISTS idx_tickets_participant_bid;
DROP INDEX IF EXISTS idx_notifications_unread_count;
DROP INDEX IF EXISTS idx_featured_campaigns_active_placement_dates;
DROP INDEX IF EXISTS idx_audit_logs_created_at;
DROP INDEX IF EXISTS idx_payout_records_agent_id;
DROP INDEX IF EXISTS idx_payout_records_status;
DROP INDEX IF EXISTS idx_agent_bank_accounts_agent_id;
DROP INDEX IF EXISTS idx_wallet_transactions_wallet_id;
DROP INDEX IF EXISTS idx_refund_records_participant_id;
DROP INDEX IF EXISTS idx_bids_auto_reject;
DROP INDEX IF EXISTS idx_winner_confirmations_pending;
DROP INDEX IF EXISTS idx_tickets_participation_sync;
-- +goose StatementEnd
