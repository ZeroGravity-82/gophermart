ALTER TABLE "order"
    ADD COLUMN lock_expires_at TIMESTAMPTZ NULL;

CREATE INDEX idx_order_lock_expires_at ON "order" (lock_expires_at);
