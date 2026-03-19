package auth

import (
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
	jwt.RegisteredClaims
	UserID uuid.UUID `json:"user_id"`
}

type KeyRing struct {
	keys [][]byte
	now  func() time.Time
}

func NewKeyRing(keys [][]byte, now func() time.Time) (*KeyRing, error) {
	if len(keys) == 0 {
		return nil, errors.New("key ring requires at least one key")
	}
	return &KeyRing{keys: keys, now: now}, nil
}

func (kr *KeyRing) Issue(userID uuid.UUID) (string, error) {
	token, err := CreateJWT(userID, kr.keys[0], kr.now())
	if err != nil {
		return "", fmt.Errorf("issuing jwt: %w", err)
	}
	return token, nil
}

func (kr *KeyRing) Verify(token string) (*Claims, error) {
	for _, key := range kr.keys {
		claims, err := ParseJWT(token, key, kr.now)
		if err == nil {
			return claims, nil
		}
		if errors.Is(err, ErrExpiredToken) {
			// we don't try other keys if the token is expired
			return nil, err
		}
	}
	return nil, ErrInvalidToken
}

func CreateJWT(userID uuid.UUID, secret []byte, now time.Time) (string, error) {
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

func ParseJWT(
	encoded string,
	secret []byte,
	nowFn func() time.Time,
) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(
		encoded,
		&Claims{},
		func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrTokenUnverifiable
			}
			return secret, nil
		},
		jwt.WithTimeFunc(nowFn),
	)
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
