package authservice

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/zanzhit/studio_recorder/internal/domain/constants"
	"github.com/zanzhit/studio_recorder/internal/domain/errs"
	"github.com/zanzhit/studio_recorder/internal/domain/models"
	jwtmid "github.com/zanzhit/studio_recorder/internal/lib/jwt"
	"github.com/zanzhit/studio_recorder/internal/lib/sl"

	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	secret       string
	tokenTTL     time.Duration
	log          *slog.Logger
	userSaver    UserSaver
	userProvider UserProvider
}

type UserSaver interface {
	SaveUser(email, userType string, passHash []byte) (string, error)
}

type UserProvider interface {
	User(email string) (models.User, error)
}

func New(log *slog.Logger, userSaver UserSaver, userProvider UserProvider, tokenTTL time.Duration, secret string) *AuthService {
	return &AuthService{
		secret:       secret,
		tokenTTL:     tokenTTL,
		log:          log,
		userSaver:    userSaver,
		userProvider: userProvider,
	}
}

func (s *AuthService) RegisterNewUser(email, password, userType string) (string, error) {
	const op = "service.auth.Register"

	log := s.log.With(
		slog.String("op", op),
		slog.String("email", email),
	)

	if userType != constants.User && userType != constants.Admin {
		s.log.Warn("invalid user_type", sl.Err(errs.ErrUserType))
		return "", fmt.Errorf("%s: %w", op, errs.ErrUserType)
	}

	log.Info("registering user")

	passHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Error("failed to hash password", sl.Err(err))

		return "", fmt.Errorf("%s: %w", op, err)
	}

	id, err := s.userSaver.SaveUser(email, userType, passHash)
	if err != nil {
		log.Error("failed to save user", sl.Err(err))

		return "", fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

func (s *AuthService) Login(email, password string) (string, error) {
	const op = "service.auth.Login"

	log := s.log.With(
		slog.String("op", op),
		slog.String("email", email),
	)

	log.Info("attempting to login user")

	user, err := s.userProvider.User(email)
	if err != nil {
		if errors.Is(err, errs.ErrInvalidCredentials) {
			s.log.Warn("user not found", sl.Err(err))

			return "", fmt.Errorf("%s: %w", op, errs.ErrInvalidCredentials)
		}

		s.log.Error("failed to get user", sl.Err(err))

		return "", fmt.Errorf("%s: %w", op, err)
	}

	if err := bcrypt.CompareHashAndPassword(user.PassHash, []byte(password)); err != nil {
		s.log.Info("invalid credentials", sl.Err(err))

		return "", fmt.Errorf("%s: %w", op, errs.ErrInvalidCredentials)
	}

	log.Info("user logged in successfully")

	token, err := jwtmid.NewToken(user, s.tokenTTL, s.secret)
	if err != nil {
		s.log.Error("failed to generate token", sl.Err(err))

		return "", fmt.Errorf("%s: %w", op, err)
	}

	return token, nil
}

func (s *AuthService) CreateInitialAdmin() error {
	const op = "service.auth.Login"

	log := s.log.With(
		slog.String("op", op),
	)

	adminEmail := os.Getenv("ADMIN_EMAIL")
	adminPassword := os.Getenv("ADMIN_PASSWORD")

	if adminEmail == "" || adminPassword == "" {
		return fmt.Errorf("ADMIN_EMAIL and ADMIN_PASSWORD are required")
	}

	_, err := s.userProvider.User(adminEmail)
	if err == nil {
		return nil
	}
	if !errors.Is(err, errs.ErrInvalidCredentials) {
		return fmt.Errorf("failed to check admin existence: %w", err)
	}

	_, err = s.RegisterNewUser(adminEmail, adminPassword, constants.Admin)
	if err != nil {
		s.log.Error("failed to create admin", sl.Err(err))

		return fmt.Errorf("failed to create admin: %w", err)
	}

	log.Info("admin created successfully")

	return nil
}
