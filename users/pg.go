package users

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jochemloedeman/misty/db/sqlc"
)

func dbUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

type UserStore struct {
	queries *sqlc.Queries
}

func NewUserStore(q *sqlc.Queries) *UserStore {
	return &UserStore{queries: q}
}

func (s *UserStore) Create(ctx context.Context, u User) (User, error) {
	id, err := s.queries.CreateUser(ctx, dbUUID(u.ID))
	if err != nil {
		return User{}, fmt.Errorf("failed to create user: %w", err)
	}
	return User{ID: uuid.UUID(id.Bytes)}, nil
}

func (s *UserStore) Ensure(ctx context.Context, u User) error {
	if err := s.queries.EnsureUser(ctx, dbUUID(u.ID)); err != nil {
		return fmt.Errorf("failed to ensure user: %w", err)
	}
	return nil
}
