CREATE TABLE "order" (
    number VARCHAR(255) PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL,
    uploaded_at TIMESTAMPTZ NOT NULL,
    accrual BIGINT NULL CHECK (accrual >= 0)
);

CREATE INDEX idx_order_user_uploaded_at ON "order" (user_id, uploaded_at DESC);
