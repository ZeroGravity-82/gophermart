CREATE TABLE withdrawal (
    order_number VARCHAR(255) PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    sum BIGINT NOT NULL CHECK (sum >= 0),
    processed_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_withdrawal_user_processed_at ON withdrawal (user_id, processed_at DESC);
