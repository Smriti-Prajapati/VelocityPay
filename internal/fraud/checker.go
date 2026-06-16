package fraud

import (
	"context"
	"fmt"
	"time"

	redisc "github.com/velocitypay/velocitypay/internal/redis"
	"go.uber.org/zap"
)

// Checker evaluates a transfer against all fraud rules.
// Rules are independent — all firing rules contribute to the risk score.
// A score >= 80 blocks the transaction; below that it is flagged but allowed.
type Checker struct {
	repo       Repository
	cache      *redisc.Client
	thresholds Thresholds
	log        *zap.Logger
}

// NewChecker creates a Checker with the given thresholds.
func NewChecker(repo Repository, cache *redisc.Client, thresholds Thresholds, log *zap.Logger) *Checker {
	return &Checker{
		repo:       repo,
		cache:      cache,
		thresholds: thresholds,
		log:        log,
	}
}

// Evaluate runs all fraud rules against the request and returns a CheckResult.
// Alerts are persisted for each rule that fires.
func (c *Checker) Evaluate(ctx context.Context, req CheckRequest) CheckResult {
	result := CheckResult{Allowed: true}

	// Run all rules
	rules := []func(context.Context, CheckRequest) *FraudAlert{
		c.ruleVelocity,
		c.ruleLargeTransaction,
		c.ruleRapidDepletion,
		c.ruleUnusualHour,
		c.ruleNewAccount,
	}

	for _, rule := range rules {
		if alert := rule(ctx, req); alert != nil {
			result.RiskScore += alert.RiskScore
			result.Alerts = append(result.Alerts, *alert)

			// Persist the alert
			if err := c.repo.CreateAlert(ctx, alert); err != nil {
				c.log.Error("failed to persist fraud alert",
					zap.String("alert_type", string(alert.AlertType)),
					zap.Error(err),
				)
			}
		}
	}

	// Cap score at 100
	if result.RiskScore > 100 {
		result.RiskScore = 100
	}

	// Block if critical risk
	if result.RiskScore >= 80 {
		result.Allowed = false
		c.log.Warn("transaction blocked by fraud engine",
			zap.String("user_id", req.UserID.String()),
			zap.String("transaction_id", req.TransactionID.String()),
			zap.Int("risk_score", result.RiskScore),
		)
	} else if result.RiskScore > 0 {
		c.log.Info("transaction flagged by fraud engine",
			zap.String("user_id", req.UserID.String()),
			zap.Int("risk_score", result.RiskScore),
		)
	}

	return result
}

// ruleVelocity checks if the user has exceeded MaxTransfersPerWindow.
// Uses Redis INCR with a sliding TTL as the counter.
func (c *Checker) ruleVelocity(ctx context.Context, req CheckRequest) *FraudAlert {
	key := fmt.Sprintf("fraud:velocity:%s", req.UserID.String())

	count, err := c.cache.Incr(ctx, key)
	if err != nil {
		return nil // fail open
	}
	if count == 1 {
		_ = c.cache.Expire(ctx, key, c.thresholds.VelocityWindow)
	}

	if count > int64(c.thresholds.MaxTransfersPerWindow) {
		return &FraudAlert{
			ID:            newID(),
			UserID:        req.UserID,
			TransactionID: req.TransactionID,
			AlertType:     AlertVelocityBreached,
			RiskLevel:     RiskHigh,
			RiskScore:     85,
			Details: fmt.Sprintf(
				"%d transfers in %s window (max %d)",
				count, c.thresholds.VelocityWindow, c.thresholds.MaxTransfersPerWindow,
			),
			CreatedAt: time.Now().UTC(),
		}
	}
	return nil
}

// ruleLargeTransaction flags transfers above the configured threshold.
func (c *Checker) ruleLargeTransaction(_ context.Context, req CheckRequest) *FraudAlert {
	if req.Amount >= c.thresholds.LargeTransactionAmount {
		return &FraudAlert{
			ID:            newID(),
			UserID:        req.UserID,
			TransactionID: req.TransactionID,
			AlertType:     AlertLargeTransaction,
			RiskLevel:     RiskMedium,
			RiskScore:     40,
			Details:       fmt.Sprintf("transfer amount ₹%.2f exceeds threshold ₹%.2f", req.Amount, c.thresholds.LargeTransactionAmount),
			CreatedAt:     time.Now().UTC(),
		}
	}
	return nil
}

// ruleRapidDepletion flags when a single transfer drains more than 80% of the wallet.
func (c *Checker) ruleRapidDepletion(_ context.Context, req CheckRequest) *FraudAlert {
	if req.WalletBalance <= 0 {
		return nil
	}
	depletionRatio := req.Amount / req.WalletBalance
	if depletionRatio >= c.thresholds.RapidDepletionPercent {
		pct := depletionRatio * 100
		return &FraudAlert{
			ID:            newID(),
			UserID:        req.UserID,
			TransactionID: req.TransactionID,
			AlertType:     AlertRapidDepletion,
			RiskLevel:     RiskMedium,
			RiskScore:     35,
			Details:       fmt.Sprintf("transfer depletes %.1f%% of wallet balance", pct),
			CreatedAt:     time.Now().UTC(),
		}
	}
	return nil
}

// ruleUnusualHour flags transfers made between 1 AM and 5 AM UTC.
func (c *Checker) ruleUnusualHour(_ context.Context, req CheckRequest) *FraudAlert {
	if req.TransferHour >= 1 && req.TransferHour <= 5 {
		return &FraudAlert{
			ID:            newID(),
			UserID:        req.UserID,
			TransactionID: req.TransactionID,
			AlertType:     AlertUnusualHour,
			RiskLevel:     RiskLow,
			RiskScore:     15,
			Details:       fmt.Sprintf("transfer at unusual hour %02d:00 UTC", req.TransferHour),
			CreatedAt:     time.Now().UTC(),
		}
	}
	return nil
}

// ruleNewAccount flags transfers from accounts less than 24h old.
func (c *Checker) ruleNewAccount(_ context.Context, req CheckRequest) *FraudAlert {
	if req.AccountAge < c.thresholds.NewAccountAgeCutoff {
		return &FraudAlert{
			ID:            newID(),
			UserID:        req.UserID,
			TransactionID: req.TransactionID,
			AlertType:     AlertNewAccountTransfer,
			RiskLevel:     RiskLow,
			RiskScore:     20,
			Details: fmt.Sprintf(
				"account is only %s old (threshold: %s)",
				req.AccountAge.Round(time.Minute), c.thresholds.NewAccountAgeCutoff,
			),
			CreatedAt: time.Now().UTC(),
		}
	}
	return nil
}
