-- +goose NO TRANSACTION
-- +goose Up

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_payments_success_unproc_retry_false 
ON payments (updated_at ASC) 
WHERE status = 'success' AND processed = false AND retry_disabled = false;

DROP INDEX IF EXISTS idx_payments_success_unprocessed;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_featured_campaigns_active_v2 
ON featured_campaigns (placement, ends_at ASC, starts_at) 
WHERE status = 'active';

DROP INDEX IF EXISTS idx_featured_campaigns_active_placement_dates;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_bids_active_count 
ON bids (status) 
WHERE deleted_at IS NULL AND status IN ('active', 'suspended');

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS idx_payments_success_unproc_retry_false;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_payments_success_unprocessed
ON payments (updated_at)
WHERE status = 'success' AND processed = false;

DROP INDEX CONCURRENTLY IF EXISTS idx_featured_campaigns_active_v2;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_featured_campaigns_active_placement_dates 
ON featured_campaigns(placement, starts_at, ends_at) 
WHERE status = 'active';

DROP INDEX CONCURRENTLY IF EXISTS idx_bids_active_count;
