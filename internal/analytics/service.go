package analytics

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	redisc "github.com/velocitypay/velocitypay/internal/redis"
	"github.com/velocitypay/velocitypay/internal/wallet"
	"go.uber.org/zap"
)

const (
	platformStatsTTL = 0 // no cache on platform stats — always fresh
)

// Service provides analytics data for users and the platform.
type Service struct {
	repo       Repository
	walletRepo wallet.Repository
	cache      *redisc.Client
	log        *zap.Logger
}

// NewService wires up the analytics service.
func NewService(repo Repository, walletRepo wallet.Repository, cache *redisc.Client, log *zap.Logger) *Service {
	return &Service{
		repo:       repo,
		walletRepo: walletRepo,
		cache:      cache,
		log:        log,
	}
}

// GetDashboard returns a combined summary for the user dashboard.
func (s *Service) GetDashboard(ctx context.Context, userID uuid.UUID) (*DashboardSummary, error) {
	stats, err := s.repo.GetUserStats(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("dashboard stats: %w", err)
	}

	w, err := s.walletRepo.FindByUserID(ctx, userID)
	if err != nil {
		// wallet may not exist yet — return partial data
		return &DashboardSummary{
			UserID:            userID.String(),
			TotalSent:         stats.TotalAmountSent,
			TotalReceived:     stats.TotalAmountReceived,
			TotalTransactions: stats.TotalTransactions,
		}, nil
	}

	return &DashboardSummary{
		UserID:            userID.String(),
		TotalSent:         stats.TotalAmountSent,
		TotalReceived:     stats.TotalAmountReceived,
		TotalTransactions: stats.TotalTransactions,
		WalletBalance:     w.Balance,
		Currency:          w.Currency,
	}, nil
}

// GetUserStats returns detailed transaction statistics for a user.
func (s *Service) GetUserStats(ctx context.Context, userID uuid.UUID) (*TransactionStats, error) {
	stats, err := s.repo.GetUserStats(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user stats: %w", err)
	}
	return stats, nil
}

// GetMonthlySpend returns monthly spending breakdown for the past N months.
func (s *Service) GetMonthlySpend(ctx context.Context, userID uuid.UUID, months int) ([]*MonthlySpend, error) {
	if months <= 0 || months > 24 {
		months = 6
	}
	data, err := s.repo.GetMonthlySpend(ctx, userID, months)
	if err != nil {
		return nil, fmt.Errorf("monthly spend: %w", err)
	}
	if data == nil {
		data = []*MonthlySpend{}
	}
	return data, nil
}

// GetDailyVolume returns daily sent/received totals for the past N days.
func (s *Service) GetDailyVolume(ctx context.Context, userID uuid.UUID, days int) ([]*DailyVolume, error) {
	if days <= 0 || days > 90 {
		days = 30
	}
	data, err := s.repo.GetDailyVolume(ctx, userID, days)
	if err != nil {
		return nil, fmt.Errorf("daily volume: %w", err)
	}
	if data == nil {
		data = []*DailyVolume{}
	}
	return data, nil
}

// GetPlatformStats returns platform-wide metrics (admin use).
func (s *Service) GetPlatformStats(ctx context.Context) (*PlatformStats, error) {
	stats, err := s.repo.GetPlatformStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("platform stats: %w", err)
	}
	s.log.Debug("platform stats fetched",
		zap.Int("total_users", stats.TotalUsers),
		zap.Int("total_transactions", stats.TotalTransactions),
		zap.Float64("total_volume", stats.TotalVolume),
	)
	return stats, nil
}

// GetTopSenders returns the top N users by amount sent.
func (s *Service) GetTopSenders(ctx context.Context, limit int) ([]*TopSender, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	senders, err := s.repo.GetTopSenders(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("top senders: %w", err)
	}
	if senders == nil {
		senders = []*TopSender{}
	}
	return senders, nil
}

// GetWalletGrowth returns daily wallet creation counts for the past N days.
func (s *Service) GetWalletGrowth(ctx context.Context, days int) ([]*WalletGrowth, error) {
	if days <= 0 || days > 90 {
		days = 30
	}
	data, err := s.repo.GetWalletGrowth(ctx, days)
	if err != nil {
		return nil, fmt.Errorf("wallet growth: %w", err)
	}
	if data == nil {
		data = []*WalletGrowth{}
	}
	return data, nil
}
