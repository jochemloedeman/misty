import "github.com/jochemloedeman/misty/monitor/sqlc"

type PostgresForecastStore struct {
	queries *sqlc.Queries
}

func NewPostgresForecastStore(queries *sqlc.Queries) *PostgresForecastStore {
	return &PostgresForecastStore{queries: queries}
}

func 