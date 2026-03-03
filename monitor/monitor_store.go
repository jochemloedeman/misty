package monitor

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jochemloedeman/misty/monitor/sqlc"
)

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

func dbTime(ts time.Time) pgtype.Timestamptz {
	if ts.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: ts, Valid: true}
}

func dbUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

type PostgresMonitorStore struct {
	queries *sqlc.Queries
}

func NewPostgresMonitorStore(q *sqlc.Queries) *PostgresMonitorStore {
	return &PostgresMonitorStore{queries: q}
}

func (s *PostgresMonitorStore) ListActive(ctx context.Context) ([]Monitor, error) {
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

func (s *PostgresMonitorStore) Create(ctx context.Context, monitor Monitor) (Monitor, error) {
	params := sqlc.CreateMonitorParams{
		ID:           dbUUID(monitor.ID),
		UserID:       dbUUID(monitor.UserID),
		IsActive:     monitor.IsActive,
		LocationName: monitor.Location.Name,
		Latitude:     monitor.Location.Lat,
		Longitude:    monitor.Location.Lon,
	}
	if monitor.ActiveAlert != nil {
		params.AlertStart = dbTime(monitor.ActiveAlert.Start)
		params.AlertEnd = dbTime(monitor.ActiveAlert.End)
	}
	row, err := s.queries.CreateMonitor(ctx, params)
	if err != nil {
		return Monitor{}, fmt.Errorf("failed to create monitor: %w", err)
	}

	return toDomainMonitor(row), nil
}

func (s *PostgresMonitorStore) UpdateAlert(ctx context.Context, monitorID uuid.UUID, alert *Alert) (Monitor, error) {
	params := sqlc.UpdateMonitorAlertParams{ID: dbUUID(monitorID)}
	if alert != nil {
		params.AlertStart = dbTime(alert.Start)
		params.AlertEnd = dbTime(alert.End)
	}
	row, err := s.queries.UpdateMonitorAlert(ctx, params)
	if err != nil {
		return Monitor{}, fmt.Errorf("failed to update monitor: %w", err)
	}
	return toDomainMonitor(row), nil
}
