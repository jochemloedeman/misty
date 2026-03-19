package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jochemloedeman/misty/db/sqlc"
)

func dbUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func dbText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

type Store struct {
	queries *sqlc.Queries
}

func NewStore(q *sqlc.Queries) *Store {
	return &Store{queries: q}
}

func toDomainUser(row sqlc.User) User {
	return User{
		ID:               uuid.UUID(row.ID.Bytes),
		PushToken:        row.PushToken.String,
		RefreshTokenHash: row.RefreshToken,
	}
}

func (s *Store) Create(ctx context.Context, user User) (User, error) {
	u := sqlc.CreateUserParams{
		ID:           dbUUID(user.ID),
		PushToken:    dbText(user.PushToken),
		RefreshToken: user.RefreshTokenHash,
	}
	dbUser, err := s.queries.CreateUser(ctx, u)
	if err != nil {
		return User{}, fmt.Errorf("creating user: %w", err)
	}
	return toDomainUser(dbUser), nil
}

func (s *Store) Ensure(ctx context.Context, u User) error {
	if err := s.queries.EnsureUser(ctx, sqlc.EnsureUserParams{
		ID:           dbUUID(u.ID),
		PushToken:    dbText(u.PushToken),
		RefreshToken: u.RefreshTokenHash,
	}); err != nil {
		return fmt.Errorf("ensuring user: %w", err)
	}
	return nil
}

func (s *Store) GetByRefreshToken(
	ctx context.Context,
	plainRefreshToken string,
) (User, error) {
	hashed := hashToken(plainRefreshToken)
	dbUser, err := s.queries.GetByRefreshToken(ctx, hashed)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("getting user by refresh token: %w", err)
	}
	return toDomainUser(dbUser), nil
}
