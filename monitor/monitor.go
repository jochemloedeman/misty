package monitor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound          = errors.New("monitor not found")
	ErrLimitReached      = errors.New("monitor limit reached")
	ErrDuplicateLocation = errors.New("duplicate monitor location")
)

type MonitorCounter interface {
	CountByUser(ctx context.Context, userID uuid.UUID) (int, error)
}

type LocationChecker interface {
	LocationExistsByUser(ctx context.Context, userID uuid.UUID, lat, lon float64) (bool, error)
}

type MonitorValidator interface {
	MonitorCounter
	LocationChecker
}

type Location struct {
	Name string
	Lat  float64
	Lon  float64
}

type RiskWindow struct {
	Start time.Time
	End   time.Time
}

type RiskWindowChangeType int

const (
	Unchanged RiskWindowChangeType = iota
	New
	Changed
	Revoked
)

type RiskWindowChange struct {
	Type       RiskWindowChangeType
	RiskWindow *RiskWindow
}

func (c RiskWindowChange) NeedsSave() bool {
	switch c.Type {
	case New, Changed, Revoked:
		return true
	default:
		return false
	}
}

func (t RiskWindowChangeType) String() string {
	switch t {
	case New:
		return "new"
	case Changed:
		return "changed"
	case Revoked:
		return "revoked"
	default:
		return "unchanged"
	}
}

func (c RiskWindowChange) NeedsNotification() bool {
	switch c.Type {
	case New:
		return true
	default:
		return false
	}
}

type ForecastHorizon struct {
	Interval time.Duration
	Steps    int
}

type Monitor struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	IsActive   bool
	Location   Location
	RiskWindow *RiskWindow
}

func NewMonitor(
	ctx context.Context,
	validator MonitorValidator,
	userID uuid.UUID,
	location Location,
	limit int,
) (Monitor, error) {
	count, err := validator.CountByUser(ctx, userID)
	if err != nil {
		return Monitor{}, fmt.Errorf("counting monitors: %w", err)
	}
	if count >= limit {
		return Monitor{}, ErrLimitReached
	}

	exists, err := validator.LocationExistsByUser(ctx, userID, location.Lat, location.Lon)
	if err != nil {
		return Monitor{}, fmt.Errorf("checking duplicate location: %w", err)
	}
	if exists {
		return Monitor{}, ErrDuplicateLocation
	}

	return Monitor{
		ID:       uuid.New(),
		UserID:   userID,
		IsActive: true,
		Location: location,
	}, nil
}

func fogRiskWindow(forecasts []Forecast, interval time.Duration) *RiskWindow {
	var start, end time.Time
	var anyFog bool
	for _, forecast := range forecasts {
		if !forecast.IsFogLikely() {
			continue
		}
		if !anyFog || forecast.Time.Before(start) {
			start = forecast.Time
		}
		if forecast.Time.After(end) {
			end = forecast.Time
		}
		anyFog = true
	}

	if !anyFog {
		return nil
	}
	return &RiskWindow{Start: start, End: end.Add(interval)}
}

func (m Monitor) ReconcileRiskWindow(
	now time.Time,
	forecasts []Forecast,
	interval time.Duration,
) (Monitor, RiskWindowChange) {
	newWindow := fogRiskWindow(forecasts, interval)

	if newWindow == nil && m.RiskWindow == nil {
		return m, RiskWindowChange{}
	}
	if newWindow == nil {
		m.RiskWindow = nil
		return m, RiskWindowChange{Type: Revoked}
	}
	if m.RiskWindow == nil || m.RiskWindow.End.Before(now) {
		m.RiskWindow = newWindow
		return m, RiskWindowChange{Type: New, RiskWindow: newWindow}
	}

	return m.reconcileExistingRiskWindow(newWindow)
}

func (m Monitor) Deactivate() Monitor {
	if !m.IsActive {
		return m
	}
	m.IsActive = false
	m.RiskWindow = nil
	return m
}

func (m Monitor) Activate() Monitor {
	if m.IsActive {
		return m
	}
	m.IsActive = true
	return m
}

func (m Monitor) reconcileExistingRiskWindow(newWindow *RiskWindow) (Monitor, RiskWindowChange) {
	if newWindow.Start.Equal(m.RiskWindow.Start) &&
		newWindow.End.Equal(m.RiskWindow.End) {
		return m, RiskWindowChange{RiskWindow: m.RiskWindow}
	}
	if newWindow.Start.After(m.RiskWindow.End) ||
		newWindow.End.Before(m.RiskWindow.Start) {
		m.RiskWindow = newWindow
		return m, RiskWindowChange{Type: New, RiskWindow: newWindow}
	}
	m.RiskWindow = newWindow
	return m, RiskWindowChange{Type: Changed, RiskWindow: newWindow}
}
