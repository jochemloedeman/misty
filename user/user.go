package user

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

const tokenByteLength = 32

var ErrNotFound = errors.New("user not found")

type User struct {
	ID               uuid.UUID
	PushToken        string
	RefreshTokenHash string
}

func generateSecretToken() (string, error) {
	b := make([]byte, tokenByteLength)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating secret token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func New() (User, string, error) {
	plainToken, err := generateSecretToken()
	if err != nil {
		return User{}, "", err
	}
	return User{
		ID:               uuid.New(),
		RefreshTokenHash: hashToken(plainToken),
	}, plainToken, nil
}
