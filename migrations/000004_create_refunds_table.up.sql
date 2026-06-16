CREATE TYPE refund_status AS ENUM ('pending', 'approved', 'rejected', 'completed');

CREATE TABLE IF NOT EXISTS refunds (
    id             UUID          PRIMARY KEY DEFAULT uuid_generate_v4(),
    transaction_id UUID          NOT NULL REFERENCES transactions(id),
    requested_by   UUID          NOT NULL REFERENCES users(id),
    amount         NUMERIC(18,2) NOT NULL CHECK (amount > 0),
    reason         TEXT          NOT NULL DEFAULT '',
    status         refund_status NOT NULL DEFAULT 'pending',
    reviewed_by    UUID          REFERENCES users(id),
    review_note    TEXT          NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_refunds_transaction_id ON refunds(transaction_id);
CREATE INDEX idx_refunds_requested_by   ON refunds(requested_by);
CREATE INDEX idx_refunds_status         ON refunds(status);
