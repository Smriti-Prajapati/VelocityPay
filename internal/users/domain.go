package users

import (
	"time"

	"github.com/google/uuid"
)

// User represents a registered account in VelocityPay.
type User struct {
	ID              uuid.UUID  `db:"id"               json:"id"`
	Name            string     `db:"name"             json:"name"`
	Email           string     `db:"email"            json:"email"`
	PasswordHash    string     `db:"password_hash"    json:"-"`
	PhoneNumber     string     `db:"phone_number"     json:"phone_number"`
	ProfileImageURL string     `db:"profile_image_url" json:"profile_image_url,omitempty"`
	IsActive        bool       `db:"is_active"        json:"is_active"`
	CreatedAt       time.Time  `db:"created_at"       json:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"       json:"updated_at"`
}

// RegisterRequest is the payload for POST /api/auth/register.
type RegisterRequest struct {
	Name        string `json:"name"         validate:"required,min=2,max=100"`
	Email       string `json:"email"        validate:"required,email"`
	Password    string `json:"password"     validate:"required,min=8"`
	PhoneNumber string `json:"phone_number" validate:"required,e164"`
}

// LoginRequest is the payload for POST /api/auth/login.
type LoginRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// UpdateProfileRequest is the payload for PUT /api/users/profile.
type UpdateProfileRequest struct {
	Name            string `json:"name"              validate:"omitempty,min=2,max=100"`
	PhoneNumber     string `json:"phone_number"      validate:"omitempty,e164"`
	ProfileImageURL string `json:"profile_image_url" validate:"omitempty,url"`
}

// ChangePasswordRequest is the payload for PUT /api/users/change-password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password"     validate:"required,min=8"`
}

// AuthResponse is returned after a successful login or registration.
type AuthResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"` // seconds
	User        *User  `json:"user"`
}
