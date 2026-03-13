package users

import (
	"errors"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("user not found")

type User struct {
	ID               uuid.UUID
	PushToken        string
	RefreshTokenHash string
}

func NewUser(pushToken string) (User, string, error) {
	plainToken, err := generateSecretToken()
	if err != nil {
		return User{}, "", err
	}
	return User{
		ID:               uuid.New(),
		PushToken:        pushToken,
		RefreshTokenHash: hashToken(plainToken),
	}, plainToken, nil
}
