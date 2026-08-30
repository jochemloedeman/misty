package monitor

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jochemloedeman/misty/db/sqlc"
	"github.com/jochemloedeman/misty/notification"
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
		Location: Location{
			Name: row.LocationName,
			Lat:  row.Latitude,
			Lon:  row.Longitude,
		},
	}
	if row.RiskWindowStart.Valid {
		m.RiskWindow = &RiskWindow{
			Start: row.RiskWindowStart.Time,
			End:   row.RiskWindowEnd.Time,
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
			Visibility:       row.Visibility,
			WeatherCode:      int(row.WeatherCode),
		},
	}
}

type pgMonitorStore struct {
	queries *sqlc.Queries
}

func NewStore(q *sqlc.Queries) *pgMonitorStore {
	return &pgMonitorStore{queries: q}
}

func (s *pgMonitorStore) ListAllActive(ctx context.Context) ([]Monitor, error) {
	rows, err := s.queries.ListActiveMonitors(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active monitors: %w", err)
	}
	monitors := make([]Monitor, len(rows))
	for i, row := range rows {
		monitors[i] = toDomainMonitor(row)
	}
	return monitors, nil
}

func (s *pgMonitorStore) ListByUser(
	ctx context.Context,
	userID uuid.UUID,
) ([]Monitor, error) {
	rows, err := s.queries.ListMonitors(ctx, dbUUID(userID))
	if err != nil {
		return nil, fmt.Errorf("list monitors: %w", err)
	}
	monitors := make([]Monitor, len(rows))
	for i, row := range rows {
		monitors[i] = toDomainMonitor(row)
	}
	return monitors, nil
}

func (s *pgMonitorStore) Get(
	ctx context.Context,
	userID uuid.UUID,
	monitorID uuid.UUID,
) (Monitor, error) {
	row, err := s.queries.GetByMonitorID(ctx, sqlc.GetByMonitorIDParams{
		ID:     dbUUID(monitorID),
		UserID: dbUUID(userID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Monitor{}, ErrNotFound
	}
	if err != nil {
		return Monitor{}, fmt.Errorf("get monitor: %w", err)
	}
	return toDomainMonitor(row), nil
}

func (s *pgMonitorStore) Create(
	ctx context.Context,
	m Monitor,
) (Monitor, error) {
	params := sqlc.CreateMonitorParams{
		ID:           dbUUID(m.ID),
		UserID:       dbUUID(m.UserID),
		IsActive:     m.IsActive,
		LocationName: m.Location.Name,
		Latitude:     m.Location.Lat,
		Longitude:    m.Location.Lon,
	}
	if m.RiskWindow != nil {
		params.RiskWindowStart = dbTime(m.RiskWindow.Start)
		params.RiskWindowEnd = dbTime(m.RiskWindow.End)
	}
	row, err := s.queries.CreateMonitor(ctx, params)
	if err != nil {
		return Monitor{}, fmt.Errorf("create monitor: %w", err)
	}
	return toDomainMonitor(row), nil
}

func (s *pgMonitorStore) Update(
	ctx context.Context,
	m Monitor,
) (Monitor, error) {
	params := sqlc.UpdateMonitorParams{
		ID:       dbUUID(m.ID),
		UserID:   dbUUID(m.UserID),
		IsActive: m.IsActive,
	}
	if m.RiskWindow != nil {
		params.RiskWindowStart = dbTime(m.RiskWindow.Start)
		params.RiskWindowEnd = dbTime(m.RiskWindow.End)
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

func (s *pgMonitorStore) Delete(
	ctx context.Context,
	userID uuid.UUID,
	monitorID uuid.UUID,
) error {
	result, err := s.queries.DeleteMonitor(ctx, sqlc.DeleteMonitorParams{
		ID:     dbUUID(monitorID),
		UserID: dbUUID(userID),
	})
	if err != nil {
		return fmt.Errorf("delete monitor: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgMonitorStore) CountByUser(
	ctx context.Context,
	userID uuid.UUID,
) (int, error) {
	count, err := s.queries.CountMonitorsByUser(ctx, dbUUID(userID))
	if err != nil {
		return 0, fmt.Errorf("count monitors: %w", err)
	}
	return int(count), nil
}

func (s *pgMonitorStore) LocationExistsByUser(
	ctx context.Context,
	userID uuid.UUID,
	lat, lon float64,
) (bool, error) {
	exists, err := s.queries.ExistsMonitorByUserAndLocation(
		ctx,
		sqlc.ExistsMonitorByUserAndLocationParams{
			UserID:    dbUUID(userID),
			Latitude:  lat,
			Longitude: lon,
		},
	)
	if err != nil {
		return false, fmt.Errorf("check duplicate location: %w", err)
	}
	return exists, nil
}

type pgForecastStore struct {
	queries *sqlc.Queries
}

func NewForecastStore(queries *sqlc.Queries) *pgForecastStore {
	return &pgForecastStore{queries: queries}
}

func (s *pgForecastStore) ListForMonitorInRange(
	ctx context.Context,
	monitorID uuid.UUID,
	from, until time.Time,
) ([]Forecast, error) {
	rows, err := s.queries.ListForecastsByMonitorIDAndHorizon(ctx,
		sqlc.ListForecastsByMonitorIDAndHorizonParams{
			MonitorID: dbUUID(monitorID),
			From:      dbTime(from),
			Until:     dbTime(until),
		})
	if err != nil {
		return nil, fmt.Errorf("list forecasts in range: %w", err)
	}
	forecasts := make([]Forecast, len(rows))
	for i, row := range rows {
		forecasts[i] = toDomainForecast(row)
	}
	return forecasts, nil
}

func dedupeByTime(forecasts []Forecast) []Forecast {
	byTime := make(map[int64]Forecast, len(forecasts))
	for _, forecast := range forecasts {
		byTime[forecast.Time.UnixNano()] = forecast
	}
	return slices.SortedFunc(
		maps.Values(byTime),
		func(a, b Forecast) int { return a.Time.Compare(b.Time) },
	)
}

func (s *pgForecastStore) Save(
	ctx context.Context,
	monitorID uuid.UUID,
	forecasts []Forecast,
) error {
	deduped := dedupeByTime(forecasts)
	params := sqlc.UpsertForecastsParams{
		MonitorID:        dbUUID(monitorID),
		ForecastAt:       make([]pgtype.Timestamptz, len(deduped)),
		Temperature:      make([]float64, len(deduped)),
		DewPoint:         make([]float64, len(deduped)),
		RelativeHumidity: make([]float64, len(deduped)),
		WindSpeed:        make([]float64, len(deduped)),
		Visibility:       make([]float64, len(deduped)),
		WeatherCode:      make([]int32, len(deduped)),
	}
	for i, forecast := range deduped {
		params.ForecastAt[i] = dbTime(forecast.Time)
		params.Temperature[i] = forecast.Temperature
		params.DewPoint[i] = forecast.DewPoint
		params.RelativeHumidity[i] = forecast.RelativeHumidity
		params.WindSpeed[i] = forecast.WindSpeed
		params.Visibility[i] = forecast.Visibility
		params.WeatherCode[i] = int32(forecast.WeatherCode)
	}
	if err := s.queries.UpsertForecasts(ctx, params); err != nil {
		return fmt.Errorf("upsert forecasts: %w", err)
	}
	return nil
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
			MonitorStore:  NewStore(queries),
			ForecastStore: NewForecastStore(queries),
			Outbox:        notification.NewOutbox(queries),
		}
		if err := fn(s); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
}
