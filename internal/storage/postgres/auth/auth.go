package authstorage

import (
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/zanzhit/studio_recorder/internal/domain/constants"
	"github.com/zanzhit/studio_recorder/internal/domain/errs"
	"github.com/zanzhit/studio_recorder/internal/domain/models"
	"github.com/zanzhit/studio_recorder/internal/storage/postgres"
)

type AuthStorage struct {
	db *sqlx.DB
}

func New(db *sqlx.DB) *AuthStorage {
	return &AuthStorage{db: db}
}

func (s *AuthStorage) SaveUser(email, userType string, passHash []byte) (string, error) {
	const op = "storage.postgres.auth.SaveUser"

	var id string
	query := fmt.Sprintf("INSERT INTO %s (email, password_hash) values ($1, $2) RETURNING id", postgres.UsersTable)

	tx, err := s.db.Begin()
	if err != nil {
		return "", fmt.Errorf("%s: %w", op, err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	row := tx.QueryRow(query, email, passHash)
	if err := row.Scan(&id); err != nil {
		return "", fmt.Errorf("%s: %w", op, err)
	}

	if userType == constants.Admin {
		adminQuery := fmt.Sprintf("INSERT INTO %s (user_id) VALUES ($1)", postgres.AdminsTable)
		if _, err := tx.Exec(adminQuery, id); err != nil {
			return "", fmt.Errorf("%s: %w", op, err)
		}
	}

	return id, nil
}

func (s *AuthStorage) User(email string) (models.User, error) {
	const op = "storage.postgres.Auth.User"

	var user models.User
	query := fmt.Sprintf("SELECT id, email, password_hash FROM %s WHERE email = $1", postgres.UsersTable)

	if err := s.db.Get(&user, query, email); err != nil {
		if err == sql.ErrNoRows {
			return models.User{}, fmt.Errorf("%s: %w", op, errs.ErrInvalidCredentials)
		}
		return models.User{}, fmt.Errorf("%s: %w", op, err)
	}

	query = fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM %s WHERE user_id = $1)", postgres.AdminsTable)
	var isAdmin bool

	err := s.db.Get(&isAdmin, query, user.Id)
	if err != nil {
		return models.User{}, fmt.Errorf("%s: %w", op, err)
	}

	if isAdmin {
		user.UserType = constants.Admin
	} else {
		user.UserType = constants.User
	}

	return user, nil
}
