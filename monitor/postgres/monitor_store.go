package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jochemloedeman/misty/monitor"
	"github.com/jochemloedeman/misty/db/sqlc"
)

func toDomainMonitor(row sqlc.Monitor) monitor.Monitor {
	m := monitor.Monitor{
		ID:       uuid.UUID(row.ID.Bytes),
		UserID:   uuid.UUID(row.UserID.Bytes),
		IsActive: row.IsActive,
		Location: monitor.Location{Name: row.LocationName, Lat: row.Latitude, Lon: row.Longitude},
	}
	if row.AlertStart.Valid {
		m.ActiveAlert = &monitor.Alert{
			Start: row.AlertStart.Time,
			End:   row.AlertEnd.Time,
		}
	}
	return m
}

type MonitorStore struct {
	queries *sqlc.Queries
}

func NewMonitorStore(q *sqlc.Queries) *MonitorStore {
	return &MonitorStore{queries: q}
}

func (s *MonitorStore) ListActive(ctx context.Context) ([]monitor.Monitor, error) {
	rows, err := s.queries.ListActiveMonitors(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list monitors: %w", err)
	}
	monitors := make([]monitor.Monitor, len(rows))
	for i, row := range rows {
		monitors[i] = toDomainMonitor(row)
	}
	return monitors, nil
}

func (s *MonitorStore) Create(ctx context.Context, m monitor.Monitor) (monitor.Monitor, error) {
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
		return monitor.Monitor{}, fmt.Errorf("failed to create monitor: %w", err)
	}

	return toDomainMonitor(row), nil
}

func (s *MonitorStore) UpdateAlert(ctx context.Context, monitorID uuid.UUID, alert *monitor.Alert) (monitor.Monitor, error) {
	params := sqlc.UpdateMonitorAlertParams{ID: dbUUID(monitorID)}
	if alert != nil {
		params.AlertStart = dbTime(alert.Start)
		params.AlertEnd = dbTime(alert.End)
	}
	row, err := s.queries.UpdateMonitorAlert(ctx, params)
	if err != nil {
		return monitor.Monitor{}, fmt.Errorf("failed to update monitor: %w", err)
	}
	return toDomainMonitor(row), nil
}
