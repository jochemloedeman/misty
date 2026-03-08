package monitor

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("monitor not found")

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

func (c AlertChange) NeedsNotification() bool {
	switch c.Type {
	case New:
		return true
	default:
		return false
	}
}

type TimeHorizon struct {
	Granularity time.Duration
	Steps       int
}

type Monitor struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	IsActive     bool
	Location     Location
	AlertHorizon TimeHorizon
	ActiveAlert  *Alert
}

func NewMonitor(userID uuid.UUID, location Location) Monitor {
	return Monitor{
		ID:       uuid.New(),
		UserID:   userID,
		IsActive: true,
		Location: location,
	}
}

func detectFog(forecasts []Forecast) *Alert {
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
	return &Alert{Start: start, End: end}
}

func (m Monitor) ReconcileAlert(now time.Time, forecasts []Forecast) AlertChange {

	newAlert := detectFog(forecasts)

	switch {
	case newAlert == nil && m.ActiveAlert == nil:
		return AlertChange{}
	case newAlert == nil && m.ActiveAlert != nil:
		return AlertChange{Type: Revoked}
	case newAlert != nil && m.ActiveAlert == nil:
		return AlertChange{Type: New, Alert: newAlert}

	case m.ActiveAlert.End.Before(now):
		return AlertChange{Type: New, Alert: newAlert}
	case newAlert.Start.Equal(m.ActiveAlert.Start) && newAlert.End.Equal(m.ActiveAlert.End):
		return AlertChange{Alert: m.ActiveAlert}
	default:
		return AlertChange{Type: Changed, Alert: newAlert}
	}
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
