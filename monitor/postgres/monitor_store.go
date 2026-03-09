package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jochemloedeman/misty/db/sqlc"
	"github.com/jochemloedeman/misty/monitor"
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

func (s *MonitorStore) ListAllActive(ctx context.Context) ([]monitor.Monitor, error) {
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

func (s *MonitorStore) Update(ctx context.Context, m monitor.Monitor) (monitor.Monitor, error) {
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
		return monitor.Monitor{}, monitor.ErrNotFound
	}
	if err != nil {
		return monitor.Monitor{}, fmt.Errorf("update monitor: %w", err)
	}
	return toDomainMonitor(row), nil
}

type ScopedMonitorStore struct {
	userID  uuid.UUID
	queries *sqlc.Queries
}

func NewScopedMonitorStore(userID uuid.UUID, q *sqlc.Queries) *ScopedMonitorStore {
	return &ScopedMonitorStore{userID: userID, queries: q}
}

func (s *ScopedMonitorStore) List(ctx context.Context) ([]monitor.Monitor, error) {
	rows, err := s.queries.ListMonitors(ctx, dbUUID(s.userID))
	if err != nil {
		return nil, fmt.Errorf("failed to list monitors: %w", err)
	}
	monitors := make([]monitor.Monitor, len(rows))
	for i, row := range rows {
		monitors[i] = toDomainMonitor(row)
	}
	return monitors, nil
}

func (s *ScopedMonitorStore) Get(ctx context.Context, monitorID uuid.UUID) (monitor.Monitor, error) {
	args := sqlc.GetByMonitorIDParams{
		ID:     dbUUID(monitorID),
		UserID: dbUUID(s.userID),
	}
	row, err := s.queries.GetByMonitorID(ctx, args)
	if errors.Is(err, pgx.ErrNoRows) {
		return monitor.Monitor{}, monitor.ErrNotFound
	}
	if err != nil {
		return monitor.Monitor{}, fmt.Errorf("failed to get monitor: %w", err)
	}
	return toDomainMonitor(row), nil
}

func (s *ScopedMonitorStore) Create(ctx context.Context, m monitor.Monitor) (monitor.Monitor, error) {
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

func (s *ScopedMonitorStore) Update(ctx context.Context, m monitor.Monitor) (monitor.Monitor, error) {
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
		return monitor.Monitor{}, monitor.ErrNotFound
	}
	if err != nil {
		return monitor.Monitor{}, fmt.Errorf("update monitor: %w", err)
	}
	return toDomainMonitor(row), nil
}

func (s *ScopedMonitorStore) Delete(ctx context.Context, monitorID uuid.UUID) error {
	result, err := s.queries.DeleteMonitor(ctx, sqlc.DeleteMonitorParams{
		ID:     dbUUID(monitorID),
		UserID: dbUUID(s.userID),
	})
	if err != nil {
		return fmt.Errorf("delete monitor: %w", err)
	}
	if result.RowsAffected() == 0 {
		return monitor.ErrNotFound
	}
	return nil
}
