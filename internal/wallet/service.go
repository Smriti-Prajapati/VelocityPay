package wallet

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"github.com/velocitypay/velocitypay/internal/metrics"
	"github.com/velocitypay/velocitypay/internal/rabbitmq"
	redisc "github.com/velocitypay/velocitypay/internal/redis"
	"go.uber.org/zap"
)

// Service contains all business logic for wallet operations.
type Service struct {
	repo      Repository
	publisher *rabbitmq.Publisher
	cache     *redisc.Client
	metrics   *metrics.Metrics
	log       *zap.Logger
}

// NewService wires up the wallet service.
func NewService(
	repo Repository,
	publisher *rabbitmq.Publisher,
	cache *redisc.Client,
	m *metrics.Metrics,
	log *zap.Logger,
) *Service {
	return &Service{
		repo:      repo,
		publisher: publisher,
		cache:     cache,
		metrics:   m,
		log:       log,
	}
}

// Create provisions a new wallet for the user.
func (s *Service) Create(ctx context.Context, userID uuid.UUID, req *CreateWalletRequest) (*Wallet, error) {
	exists, err := s.repo.ExistsByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("check wallet exists: %w", err)
	}
	if exists {
		return nil, ErrAlreadyExists
	}

	now := time.Now().UTC()
	w := &Wallet{
		ID:           uuid.New(),
		UserID:       userID,
		Balance:      0,
		WalletNumber: generateWalletNumber(),
		Currency:     req.Currency,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.repo.Create(ctx, w); err != nil {
		return nil, fmt.Errorf("create wallet: %w", err)
	}

	// Fire-and-forget event
	_ = s.publisher.Publish(ctx, rabbitmq.EventWalletCreated, map[string]interface{}{
		"wallet_id": w.ID,
		"user_id":   w.UserID,
		"currency":  w.Currency,
	})

	s.metrics.WalletsTotal.Inc()
	s.log.Info("wallet created", zap.String("wallet_id", w.ID.String()), zap.String("user_id", userID.String()))
	return w, nil
}

// AddMoney credits the wallet balance and records a deposit transaction.
func (s *Service) AddMoney(ctx context.Context, userID uuid.UUID, req *AddMoneyRequest) (*Wallet, error) {
	w, err := s.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("find wallet: %w", err)
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = s.repo.UpdateBalance(ctx, tx, w.ID, req.Amount); err != nil {
		return nil, fmt.Errorf("credit balance: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit add-money: %w", err)
	}

	// Return the refreshed wallet
	refreshed, err := s.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("refresh wallet: %w", err)
	}

	// Invalidate cached balance
	_ = s.cache.Del(ctx, "wallet:"+userID.String())

	s.log.Info("money added to wallet",
		zap.String("wallet_id", w.ID.String()),
		zap.Float64("amount", req.Amount),
	)
	return refreshed, nil
}

// GetBalance returns the current wallet for a user.
func (s *Service) GetBalance(ctx context.Context, userID uuid.UUID) (*Wallet, error) {
	w, err := s.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("find wallet: %w", err)
	}
	return w, nil
}

// GetDetails returns the wallet with aggregated transaction totals.
func (s *Service) GetDetails(ctx context.Context, userID uuid.UUID) (*WalletDetails, error) {
	w, err := s.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("find wallet: %w", err)
	}

	sent, err := s.repo.TotalSentByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("total sent: %w", err)
	}

	received, err := s.repo.TotalReceivedByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("total received: %w", err)
	}

	return &WalletDetails{
		Wallet:        w,
		TotalSent:     sent,
		TotalReceived: received,
	}, nil
}

// generateWalletNumber produces a unique 12-digit wallet number.
func generateWalletNumber() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("VPY%09d", r.Intn(1_000_000_000))
}
