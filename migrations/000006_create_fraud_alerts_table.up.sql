CREATE TABLE IF NOT EXISTS fraud_alerts (
    id             UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id        UUID        NOT NULL REFERENCES users(id),
    transaction_id UUID        NOT NULL,
    alert_type     VARCHAR(50) NOT NULL,
    risk_level     VARCHAR(20) NOT NULL,
    risk_score     INT         NOT NULL DEFAULT 0 CHECK (risk_score BETWEEN 0 AND 100),
    details        TEXT        NOT NULL DEFAULT '',
    reviewed       BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_fraud_alerts_user_id    ON fraud_alerts(user_id);
CREATE INDEX idx_fraud_alerts_risk_score ON fraud_alerts(risk_score DESC);
CREATE INDEX idx_fraud_alerts_reviewed   ON fraud_alerts(reviewed) WHERE reviewed = FALSE;
