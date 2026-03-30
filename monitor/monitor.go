package monitor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound     = errors.New("monitor not found")
	ErrLimitReached = errors.New("monitor limit reached")
)

type MonitorCounter interface {
	CountByUser(ctx context.Context, userID uuid.UUID) (int, error)
}

type Location struct {
	Name string
	Lat  float64
	Lon  float64
}

type Alert struct {
	Start time.Time
	End   time.Time
}

type AlertChangeType int

const (
	Unchanged AlertChangeType = iota
	New
	Changed
	Revoked
)

type AlertChange struct {
	Type  AlertChangeType
	Alert *Alert
}

func (c AlertChange) NeedsSave() bool {
	switch c.Type {
	case New, Changed, Revoked:
		return true
	default:
		return false
	}
}

func (t AlertChangeType) String() string {
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

func (c AlertChange) NeedsNotification() bool {
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
	ID          uuid.UUID
	UserID      uuid.UUID
	IsActive    bool
	Location    Location
	ActiveAlert *Alert
}

func NewMonitor(
	ctx context.Context,
	counter MonitorCounter,
	userID uuid.UUID,
	location Location,
	limit int,
) (Monitor, error) {
	count, err := counter.CountByUser(ctx, userID)
	if err != nil {
		return Monitor{}, fmt.Errorf("counting monitors: %w", err)
	}
	if count >= limit {
		return Monitor{}, ErrLimitReached
	}
	return Monitor{
		ID:       uuid.New(),
		UserID:   userID,
		IsActive: true,
		Location: location,
	}, nil
}

func fogAlert(forecasts []Forecast, interval time.Duration) *Alert {
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
	return &Alert{Start: start, End: end.Add(interval)}
}

func (m Monitor) ReconcileAlert(
	now time.Time,
	forecasts []Forecast,
	interval time.Duration,
) (Monitor, AlertChange) {
	newAlert := fogAlert(forecasts, interval)

	if newAlert == nil && m.ActiveAlert == nil {
		return m, AlertChange{}
	}
	if newAlert == nil {
		m.ActiveAlert = nil
		return m, AlertChange{Type: Revoked}
	}
	if m.ActiveAlert == nil || m.ActiveAlert.End.Before(now) {
		m.ActiveAlert = newAlert
		return m, AlertChange{Type: New, Alert: newAlert}
	}

	// now we know both newAlert and m.ActiveAlert
	// are non-nil and the active alert is still valid
	return m.reconcileExistingAlert(newAlert)
}

func (m Monitor) Deactivate() Monitor {
	if !m.IsActive {
		return m
	}
	m.IsActive = false
	m.ActiveAlert = nil
	return m
}

func (m Monitor) Activate() Monitor {
	if m.IsActive {
		return m
	}
	m.IsActive = true
	return m
}

func (m Monitor) reconcileExistingAlert(newAlert *Alert) (Monitor, AlertChange) {
	if newAlert.Start.Equal(m.ActiveAlert.Start) &&
		newAlert.End.Equal(m.ActiveAlert.End) {
		return m, AlertChange{Alert: m.ActiveAlert}
	}
	if newAlert.Start.After(m.ActiveAlert.End) ||
		newAlert.End.Before(m.ActiveAlert.Start) {
		m.ActiveAlert = newAlert
		return m, AlertChange{Type: New, Alert: newAlert}
	}
	m.ActiveAlert = newAlert
	return m, AlertChange{Type: Changed, Alert: newAlert}
}
