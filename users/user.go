package users

import (
	"errors"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("user not found")

type User struct {
	ID           uuid.UUID
	PushToken    string
	RefreshToken string
}

func NewUser(pushToken string) (User, string, error) {
	r, err := generateSecretToken()
	if err != nil {
		return User{}, "", err
	}
	hashedToken := hashToken(r)
	return User{
		ID:           uuid.New(),
		PushToken:    pushToken,
		RefreshToken: hashedToken,
	}, r, nil
}
