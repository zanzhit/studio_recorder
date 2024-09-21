package postgres

import (
	"fmt"
	"os"

	"github.com/jmoiron/sqlx"
	"github.com/zanzhit/studio_recorder/internal/config"
)

func New(cfg config.DB) (*sqlx.DB, error) {
	const op = "storage.postgres.New"

	password := os.Getenv("POSTGRES_PASSWORD")
	if password == "" {
		panic("POSTGRES_PASSWORD is required")
	}

	db, err := sqlx.Open("postgres", fmt.Sprintf("host=%s port=%s user=%s dbname=%s password=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.Username, cfg.DBName, password, cfg.SSLMode),
	)

	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return db, nil
}
