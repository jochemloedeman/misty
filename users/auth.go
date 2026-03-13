package users

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken = errors.New("invalid jwt token")
	ErrExpiredToken = errors.New("jwt token has expired")
)

type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	jwt.RegisteredClaims
}

func generateSecretToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating secret token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

type KeyRing struct {
	keys [][]byte
}

func NewKeyRing(keys [][]byte) *KeyRing {
	if len(keys) == 0 {
		panic("users: NewKeyRing requires at least one key")
	}
	return &KeyRing{keys: keys}
}

func (kr *KeyRing) Issue(userID uuid.UUID) (string, error) {
	token, err := CreateJWT(userID, kr.keys[0])
	if err != nil {
		return "", fmt.Errorf("issuing jwt: %w", err)
	}
	return token, nil
}

func (kr *KeyRing) Verify(token string) (*Claims, error) {
	for _, key := range kr.keys {
		claims, err := ParseJWT(token, key)
		if err == nil {
			return claims, nil
		}
		if errors.Is(err, ErrExpiredToken) {
			return nil, err
		}
	}
	return nil, ErrInvalidToken
}

func CreateJWT(userID uuid.UUID, secret []byte) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "misty",
			Subject:   userID.String(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

func ParseJWT(encoded string, secret []byte) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(encoded, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrTokenUnverifiable
		}
		return secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}
	if claims, ok := parsed.Claims.(*Claims); ok && parsed.Valid {
		return claims, nil
	}
	return nil, ErrInvalidToken
}
