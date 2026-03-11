package monitor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jochemloedeman/misty/db/sqlc"
	"github.com/jochemloedeman/misty/notifications"
)

func dbTime(ts time.Time) pgtype.Timestamptz {
	if ts.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: ts, Valid: true}
}

func dbUUID(id uuid.UUID) pgtype.UUID {
	if id == uuid.Nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}

func toDomainMonitor(row sqlc.Monitor) Monitor {
	m := Monitor{
		ID:       uuid.UUID(row.ID.Bytes),
		UserID:   uuid.UUID(row.UserID.Bytes),
		IsActive: row.IsActive,
		Location: Location{Name: row.LocationName, Lat: row.Latitude, Lon: row.Longitude},
	}
	if row.AlertStart.Valid {
		m.ActiveAlert = &Alert{
			Start: row.AlertStart.Time,
			End:   row.AlertEnd.Time,
		}
	}
	return m
}

func toDomainForecast(row sqlc.Forecast) Forecast {
	return Forecast{
		Time: row.ForecastAt.Time,
		WeatherVariables: WeatherVariables{
			Temperature:      row.Temperature,
			DewPoint:         row.DewPoint,
			RelativeHumidity: row.RelativeHumidity,
			WindSpeed:        row.WindSpeed,
			Visibility:       int(row.Visibility),
		},
	}
}

// pgMonitorStore implements MonitorStore backed by PostgreSQL.
type pgMonitorStore struct {
	queries *sqlc.Queries
}

func NewMonitorStore(q *sqlc.Queries) *pgMonitorStore {
	return &pgMonitorStore{queries: q}
}

func (s *pgMonitorStore) ListAllActive(ctx context.Context) ([]Monitor, error) {
	rows, err := s.queries.ListActiveMonitors(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list monitors: %w", err)
	}
	monitors := make([]Monitor, len(rows))
	for i, row := range rows {
		monitors[i] = toDomainMonitor(row)
	}
	return monitors, nil
}

func (s *pgMonitorStore) Update(ctx context.Context, m Monitor) (Monitor, error) {
	params := sqlc.UpdateMonitorByIDParams{
		ID:       dbUUID(m.ID),
		IsActive: m.IsActive,
	}
	if m.ActiveAlert != nil {
		params.AlertStart = dbTime(m.ActiveAlert.Start)
		params.AlertEnd = dbTime(m.ActiveAlert.End)
	}
	row, err := s.queries.UpdateMonitorByID(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return Monitor{}, ErrNotFound
	}
	if err != nil {
		return Monitor{}, fmt.Errorf("update monitor: %w", err)
	}
	return toDomainMonitor(row), nil
}

// pgScopedMonitorStore implements a user-scoped monitor store backed by PostgreSQL.
type pgScopedMonitorStore struct {
	userID  uuid.UUID
	queries *sqlc.Queries
}

func NewScopedMonitorStore(userID uuid.UUID, q *sqlc.Queries) *pgScopedMonitorStore {
	return &pgScopedMonitorStore{userID: userID, queries: q}
}

func (s *pgScopedMonitorStore) List(ctx context.Context) ([]Monitor, error) {
	rows, err := s.queries.ListMonitors(ctx, dbUUID(s.userID))
	if err != nil {
		return nil, fmt.Errorf("failed to list monitors: %w", err)
	}
	monitors := make([]Monitor, len(rows))
	for i, row := range rows {
		monitors[i] = toDomainMonitor(row)
	}
	return monitors, nil
}

func (s *pgScopedMonitorStore) Get(ctx context.Context, monitorID uuid.UUID) (Monitor, error) {
	args := sqlc.GetByMonitorIDParams{
		ID:     dbUUID(monitorID),
		UserID: dbUUID(s.userID),
	}
	row, err := s.queries.GetByMonitorID(ctx, args)
	if errors.Is(err, pgx.ErrNoRows) {
		return Monitor{}, ErrNotFound
	}
	if err != nil {
		return Monitor{}, fmt.Errorf("failed to get monitor: %w", err)
	}
	return toDomainMonitor(row), nil
}

func (s *pgScopedMonitorStore) Create(ctx context.Context, m Monitor) (Monitor, error) {
	params := sqlc.CreateMonitorParams{
		ID:           dbUUID(m.ID),
		UserID:       dbUUID(m.UserID),
		IsActive:     m.IsActive,
		LocationName: m.Location.Name,
		Latitude:     m.Location.Lat,
		Longitude:    m.Location.Lon,
	}
	if m.ActiveAlert != nil {
		params.AlertStart = dbTime(m.ActiveAlert.Start)
		params.AlertEnd = dbTime(m.ActiveAlert.End)
	}
	row, err := s.queries.CreateMonitor(ctx, params)
	if err != nil {
		return Monitor{}, fmt.Errorf("failed to create monitor: %w", err)
	}
	return toDomainMonitor(row), nil
}

func (s *pgScopedMonitorStore) Update(ctx context.Context, m Monitor) (Monitor, error) {
	params := sqlc.UpdateMonitorParams{
		ID:       dbUUID(m.ID),
		UserID:   dbUUID(s.userID),
		IsActive: m.IsActive,
	}
	if m.ActiveAlert != nil {
		params.AlertStart = dbTime(m.ActiveAlert.Start)
		params.AlertEnd = dbTime(m.ActiveAlert.End)
	}
	row, err := s.queries.UpdateMonitor(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return Monitor{}, ErrNotFound
	}
	if err != nil {
		return Monitor{}, fmt.Errorf("update monitor: %w", err)
	}
	return toDomainMonitor(row), nil
}

func (s *pgScopedMonitorStore) Delete(ctx context.Context, monitorID uuid.UUID) error {
	result, err := s.queries.DeleteMonitor(ctx, sqlc.DeleteMonitorParams{
		ID:     dbUUID(monitorID),
		UserID: dbUUID(s.userID),
	})
	if err != nil {
		return fmt.Errorf("delete monitor: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// pgForecastStore implements ForecastStore backed by PostgreSQL.
type pgForecastStore struct {
	queries *sqlc.Queries
}

func NewForecastStore(queries *sqlc.Queries) *pgForecastStore {
	return &pgForecastStore{queries: queries}
}

func (s *pgForecastStore) ListForMonitor(ctx context.Context, monitorID uuid.UUID) ([]Forecast, error) {
	rows, err := s.queries.ListForecastsByMonitorID(ctx, dbUUID(monitorID))
	if err != nil {
		return nil, fmt.Errorf("failed to list forecasts: %w", err)
	}
	forecasts := make([]Forecast, len(rows))
	for i, row := range rows {
		forecasts[i] = toDomainForecast(row)
	}
	return forecasts, nil
}

func (s *pgForecastStore) Save(ctx context.Context, monitorID uuid.UUID, forecasts []Forecast) ([]Forecast, error) {
	params := make([]sqlc.UpsertForecastParams, len(forecasts))
	for i, forecast := range forecasts {
		params[i] = sqlc.UpsertForecastParams{
			ForecastAt:       dbTime(forecast.Time),
			Temperature:      forecast.WeatherVariables.Temperature,
			DewPoint:         forecast.WeatherVariables.DewPoint,
			RelativeHumidity: forecast.WeatherVariables.RelativeHumidity,
			WindSpeed:        forecast.WeatherVariables.WindSpeed,
			Visibility:       int32(forecast.WeatherVariables.Visibility),
			MonitorID:        dbUUID(monitorID),
		}
	}
	savedForecasts := make([]Forecast, len(forecasts))

	for i := range params {
		row, err := s.queries.UpsertForecast(ctx, params[i])
		if err != nil {
			return nil, fmt.Errorf("failed to upsert forecast: %w", err)
		}
		savedForecasts[i] = toDomainForecast(row)
	}

	return savedForecasts, nil
}

func NewRunAtomically(pool *pgxpool.Pool) RunAtomically {
	return func(ctx context.Context, fn func(s AtomicStores) error) error {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback(ctx) }()

		queries := sqlc.New(tx)

		s := AtomicStores{
			MonitorStore:  NewMonitorStore(queries),
			ForecastStore: NewForecastStore(queries),
			Outbox:        notifications.NewOutbox(queries),
		}
		if err := fn(s); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
}
