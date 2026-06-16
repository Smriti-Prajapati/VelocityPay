package database

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/velocitypay/velocitypay/internal/config"
	"go.uber.org/zap"
)

// NewPostgresDB opens a connection pool to PostgreSQL and verifies connectivity.
func NewPostgresDB(cfg *config.DatabaseConfig, log *zap.Logger) (*sqlx.DB, error) {
	db, err := sqlx.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxConns)
	db.SetMaxIdleConns(cfg.MaxIdle)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	log.Info("connected to PostgreSQL",
		zap.String("host", cfg.Host),
		zap.String("db", cfg.Name),
	)
	return db, nil
}
