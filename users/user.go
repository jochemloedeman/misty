package users

import (
	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID
	PushToken    string
	RefreshToken string
}

func NewUser(pushToken string) (User, string) {
	r := generateSecretToken()
	hashedToken := hashToken(r)
	return User{
		ID:           uuid.New(),
		PushToken:    pushToken,
		RefreshToken: hashedToken,
	}, r
}
