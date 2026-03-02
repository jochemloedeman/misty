package monitor

import (
	"time"

	"github.com/google/uuid"
)

type Location struct {
	Lat float64
	Lon float64
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

type Monitor struct {
	ID       uuid.UUID
	IsActive bool
	Location Location
	Alert    *Alert
}

func fogAlert(forecasts []Forecast) *Alert {
	var start, end time.Time
	var anyFog bool
	for _, forecast := range forecasts {
		if !forecast.FogIsLikely() {
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

func (m Monitor) EvaluateAlert(forecasts []Forecast) AlertChange {
	if !m.IsActive {
		if m.Alert != nil {
			return AlertChange{Type: Revoked}
		}
		return AlertChange{}
	}

	newAlert := fogAlert(forecasts)

	switch {
	case newAlert == nil && m.Alert == nil:
		return AlertChange{}
	case newAlert == nil && m.Alert != nil:
		return AlertChange{Type: Revoked}
	case newAlert != nil && m.Alert == nil:
		return AlertChange{Type: New, Alert: newAlert}
	case newAlert.Start.Equal(m.Alert.Start) && newAlert.End.Equal(m.Alert.End):
		return AlertChange{Alert: m.Alert}
	default:
		return AlertChange{Type: Changed, Alert: newAlert}
	}
}
