CREATE TYPE transaction_status AS ENUM ('pending', 'completed', 'failed', 'reversed');
CREATE TYPE transaction_type   AS ENUM ('transfer', 'deposit', 'refund');

CREATE TABLE IF NOT EXISTS transactions (
    id               UUID               PRIMARY KEY DEFAULT uuid_generate_v4(),
    sender_id        UUID               NOT NULL REFERENCES users(id),
    receiver_id      UUID               NOT NULL REFERENCES users(id),
    amount           NUMERIC(18, 2)     NOT NULL CHECK (amount > 0),
    transaction_type transaction_type   NOT NULL DEFAULT 'transfer',
    status           transaction_status NOT NULL DEFAULT 'pending',
    notes            TEXT               NOT NULL DEFAULT '',
    idempotency_key  VARCHAR(64)        UNIQUE,
    failure_reason   TEXT               NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ        NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ        NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_transactions_sender_id   ON transactions(sender_id);
CREATE INDEX idx_transactions_receiver_id ON transactions(receiver_id);
CREATE INDEX idx_transactions_status      ON transactions(status);
CREATE INDEX idx_transactions_created_at  ON transactions(created_at DESC);
CREATE INDEX idx_transactions_idem_key    ON transactions(idempotency_key) WHERE idempotency_key IS NOT NULL;
