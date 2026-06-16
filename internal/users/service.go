package users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/velocitypay/velocitypay/internal/auth"
	"github.com/velocitypay/velocitypay/internal/rabbitmq"
	redisc "github.com/velocitypay/velocitypay/internal/redis"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

const (
	userCacheTTL    = 5 * time.Minute
	userCachePrefix = "user:"
)

// Service contains all business logic for user accounts.
type Service struct {
	repo      Repository
	tokens    *auth.TokenManager
	cache     *redisc.Client
	publisher *rabbitmq.Publisher
	log       *zap.Logger
}

// NewService wires up the user service.
func NewService(repo Repository, tokens *auth.TokenManager, cache *redisc.Client, publisher *rabbitmq.Publisher, log *zap.Logger) *Service {
	return &Service{
		repo:      repo,
		tokens:    tokens,
		cache:     cache,
		publisher: publisher,
		log:       log,
	}
}

// Register creates a new user account and returns an access token.
func (s *Service) Register(ctx context.Context, req *RegisterRequest) (*AuthResponse, error) {
	exists, err := s.repo.ExistsByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("check email: %w", err)
	}
	if exists {
		return nil, ErrEmailTaken
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	now := time.Now().UTC()
	u := &User{
		ID:           uuid.New(),
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: string(hash),
		PhoneNumber:  req.PhoneNumber,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.repo.Create(ctx, u); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	token, expiry, err := s.tokens.Issue(u.ID, u.Email)
	if err != nil {
		return nil, fmt.Errorf("issue token: %w", err)
	}

	// Publish event so notification service sends a welcome message
	_ = s.publisher.Publish(ctx, rabbitmq.EventUserRegistered, map[string]interface{}{
		"user_id": u.ID,
		"name":    u.Name,
		"email":   u.Email,
	})

	s.log.Info("user registered",
		zap.String("user_id", u.ID.String()),
		zap.String("email", u.Email),
	)

	return &AuthResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int64(time.Until(expiry).Seconds()),
		User:        u,
	}, nil
}

// Login authenticates credentials and returns an access token.
func (s *Service) Login(ctx context.Context, req *LoginRequest) (*AuthResponse, error) {
	u, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid credentials")
	}

	if !u.IsActive {
		return nil, errors.New("account is deactivated")
	}

	token, expiry, err := s.tokens.Issue(u.ID, u.Email)
	if err != nil {
		return nil, fmt.Errorf("issue token: %w", err)
	}

	s.log.Info("user logged in", zap.String("user_id", u.ID.String()))

	return &AuthResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int64(time.Until(expiry).Seconds()),
		User:        u,
	}, nil
}

// GetProfile returns the user profile.
func (s *Service) GetProfile(ctx context.Context, userID uuid.UUID) (*User, error) {
	u, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}
	return u, nil
}

// UpdateProfile updates mutable profile fields.
func (s *Service) UpdateProfile(ctx context.Context, userID uuid.UUID, req *UpdateProfileRequest) (*User, error) {
	u, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("find user: %w", err)
	}

	if req.Name != "" {
		u.Name = req.Name
	}
	if req.PhoneNumber != "" {
		u.PhoneNumber = req.PhoneNumber
	}
	if req.ProfileImageURL != "" {
		u.ProfileImageURL = req.ProfileImageURL
	}
	u.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, u); err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}

	_ = s.cache.Del(ctx, userCachePrefix+userID.String())

	s.log.Info("profile updated", zap.String("user_id", userID.String()))
	return u, nil
}

// ChangePassword validates the current password and sets a new hash.
func (s *Service) ChangePassword(ctx context.Context, userID uuid.UUID, req *ChangePasswordRequest) error {
	u, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("find user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		return errors.New("current password is incorrect")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.repo.UpdatePassword(ctx, userID, string(hash)); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	s.log.Info("password changed", zap.String("user_id", userID.String()))
	return nil
}
