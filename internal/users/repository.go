package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// ErrNotFound is returned when a user cannot be found.
var ErrNotFound = errors.New("user not found")

// ErrEmailTaken is returned when email is already registered.
var ErrEmailTaken = errors.New("email already in use")

// Repository defines the data access contract for users.
type Repository interface {
	Create(ctx context.Context, u *User) error
	FindByID(ctx context.Context, id uuid.UUID) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, u *User) error
	UpdatePassword(ctx context.Context, id uuid.UUID, hash string) error
	ExistsByEmail(ctx context.Context, email string) (bool, error)
}

type postgresRepository struct {
	db *sqlx.DB
}

// NewRepository creates a PostgreSQL-backed user repository.
func NewRepository(db *sqlx.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) Create(ctx context.Context, u *User) error {
	query := `
		INSERT INTO users (id, name, email, password_hash, phone_number, profile_image_url, is_active, created_at, updated_at)
		VALUES (:id, :name, :email, :password_hash, :phone_number, :profile_image_url, :is_active, :created_at, :updated_at)
	`
	if _, err := r.db.NamedExecContext(ctx, query, u); err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *postgresRepository) FindByID(ctx context.Context, id uuid.UUID) (*User, error) {
	u := &User{}
	err := r.db.GetContext(ctx, u, `SELECT * FROM users WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user by id: %w", err)
	}
	return u, nil
}

func (r *postgresRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	u := &User{}
	err := r.db.GetContext(ctx, u, `SELECT * FROM users WHERE email = $1`, email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user by email: %w", err)
	}
	return u, nil
}

func (r *postgresRepository) Update(ctx context.Context, u *User) error {
	query := `
		UPDATE users
		SET name = :name,
		    phone_number = :phone_number,
		    profile_image_url = :profile_image_url,
		    updated_at = :updated_at
		WHERE id = :id
	`
	res, err := r.db.NamedExecContext(ctx, query, u)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *postgresRepository) UpdatePassword(ctx context.Context, id uuid.UUID, hash string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2`,
		hash, id,
	)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *postgresRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, email,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check email exists: %w", err)
	}
	return exists, nil
}
