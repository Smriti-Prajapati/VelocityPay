CREATE TABLE IF NOT EXISTS wallets (
    id            UUID           PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id       UUID           NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    balance       NUMERIC(18, 2) NOT NULL DEFAULT 0.00 CHECK (balance >= 0),
    wallet_number VARCHAR(20)    NOT NULL UNIQUE,
    currency      VARCHAR(3)     NOT NULL DEFAULT 'INR',
    is_active     BOOLEAN        NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_wallets_user_id    ON wallets(user_id);
CREATE INDEX         idx_wallets_number    ON wallets(wallet_number);
