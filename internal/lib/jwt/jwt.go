package jwt

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/zanzhit/studio_recorder/internal/domain/models"
)

func NewToken(user models.User, duration time.Duration, secret string) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["uid"] = user.Id
	claims["email"] = user.Email
	claims["exp"] = time.Now().Add(duration).Unix()
	claims["user_type"] = user.UserType

	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
