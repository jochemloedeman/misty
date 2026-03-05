package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jochemloedeman/misty/users"
	"github.com/jochemloedeman/misty/users/postgres/sqlc"
)

type UserStore struct {
	queries *sqlc.Queries
}

func NewUserStore(q *sqlc.Queries) *UserStore {
	return &UserStore{queries: q}
}

func (s *UserStore) Create(ctx context.Context, u users.User) (users.User, error) {
	id, err := s.queries.CreateUser(ctx, dbUUID(u.ID))
	if err != nil {
		return users.User{}, fmt.Errorf("failed to create user: %w", err)
	}
	return users.User{ID: uuid.UUID(id.Bytes)}, nil
}

func dbUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}
