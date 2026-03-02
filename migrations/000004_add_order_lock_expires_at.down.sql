DROP INDEX IF EXISTS idx_order_lock_expires_at;

ALTER TABLE "order"
    DROP COLUMN IF EXISTS lock_expires_at;
