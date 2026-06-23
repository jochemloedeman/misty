package apple

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jochemloedeman/misty/db/sqlc"
)

// PGTokenResolver looks up APNs device tokens from PostgreSQL.
type PGTokenResolver struct {
	queries *sqlc.Queries
}

func NewPGTokenResolver(q *sqlc.Queries) *PGTokenResolver {
	return &PGTokenResolver{queries: q}
}

func (r *PGTokenResolver) PushToken(
	ctx context.Context,
	userID uuid.UUID,
) (string, error) {
	token, err := r.queries.GetPushTokenByUserID(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("user %s not found: %w", userID, err)
		}
		return "", fmt.Errorf("query push token: %w", err)
	}
	return token.String, nil
}

func (r *PGTokenResolver) ClearPushToken(
	ctx context.Context,
	userID uuid.UUID,
	token string,
) error {
	err := r.queries.ClearPushToken(ctx, sqlc.ClearPushTokenParams{
		ID:        pgtype.UUID{Bytes: userID, Valid: true},
		PushToken: pgtype.Text{String: token, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("clear push token for %s: %w", userID, err)
	}
	return nil
}
