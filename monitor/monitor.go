package monitor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const maxSeparation = time.Hour

var (
	ErrNotFound          = errors.New("monitor not found")
	ErrLimitReached      = errors.New("monitor limit reached")
	ErrDuplicateLocation = errors.New("duplicate monitor location")
)

type MonitorCounter interface {
	CountByUser(ctx context.Context, userID uuid.UUID) (int, error)
}

type LocationChecker interface {
	LocationExistsByUser(
		ctx context.Context,
		userID uuid.UUID,
		lat, lon float64,
	) (bool, error)
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

func (r RiskWindow) Equal(w RiskWindow) bool {
	return r.Start.Equal(w.Start) && r.End.Equal(w.End)
}

func (r RiskWindow) Disjoint(w RiskWindow, margin time.Duration) bool {
	return r.Start.After(w.End.Add(margin)) || r.End.Before(w.Start.Add(-margin))
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

// NOTE: calculates a concrete end time, even when the last
// timestamp suggests fog.
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

type windowTransition int

const (
	stayClear windowTransition = iota
	cleared
	appeared
	stable
	replaced
	shifted
)

func classifyTransition(o, n *RiskWindow, now time.Time) windowTransition {
	if n == nil {
		if o == nil {
			return stayClear
		}
		return cleared
	}

	if o == nil || o.End.Before(now) {
		return appeared
	}

	// both windows present and the old one is still live
	switch {
	case n.Equal(*o):
		return stable
	case n.Disjoint(*o, maxSeparation):
		return replaced
	default:
		return shifted
	}
}

func (m Monitor) ReconcileRiskWindow(
	now time.Time,
	forecasts []Forecast,
	interval time.Duration,
) (Monitor, RiskWindowChange) {
	newWindow := fogRiskWindow(forecasts, interval)
	switch classifyTransition(m.RiskWindow, newWindow, now) {
	case cleared:
		m.RiskWindow = nil
		return m, RiskWindowChange{Type: Revoked}
	case appeared, replaced:
		m.RiskWindow = newWindow
		return m, RiskWindowChange{Type: New, RiskWindow: newWindow}
	case shifted:
		m.RiskWindow = newWindow
		return m, RiskWindowChange{Type: Changed, RiskWindow: newWindow}
	case stable:
		return m, RiskWindowChange{RiskWindow: m.RiskWindow}
	default:
		return m, RiskWindowChange{}
	}
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
