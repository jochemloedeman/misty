package monitor

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
)

var defaultUUID = uuid.MustParse("00000000-0000-0000-0000-000000000000")
var defaultLocation = Location{
	Name: "Test Location",
	Lat:  0.0,
	Lon:  0.0,
}

var defaultTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
var noFogVariables = WeatherVariables{
	Temperature:      20,
	DewPoint:         10,
	RelativeHumidity: 50,
	WindSpeed:        5,
	Visibility:       10000,
}

var fogVariables = WeatherVariables{
	Temperature:      10,
	DewPoint:         9,
	RelativeHumidity: 98,
	WindSpeed:        2,
	Visibility:       500,
}

func TestReconcileAlert(t *testing.T) {
	testCases := []struct {
		name      string
		monitor   Monitor
		forecasts []Forecast
		expected  AlertChange
	}{
		{
			name: "clear - no current alert",
			monitor: Monitor{
				ID:       defaultUUID,
				UserID:   defaultUUID,
				IsActive: true,
				Location: defaultLocation,
			},
			forecasts: []Forecast{
				{
					Time:             defaultTime,
					WeatherVariables: noFogVariables,
				},
				{
					Time:             defaultTime.Add(1 * time.Hour),
					WeatherVariables: noFogVariables,
				},
				{
					Time:             defaultTime.Add(2 * time.Hour),
					WeatherVariables: noFogVariables,
				},
			},
			expected: AlertChange{
				Type:  Unchanged,
				Alert: nil,
			},
		},
		{
			name: "clear - existing alert",
			monitor: Monitor{
				ID:       defaultUUID,
				UserID:   defaultUUID,
				IsActive: true,
				Location: defaultLocation,
				ActiveAlert: &Alert{
					Start: defaultTime,
					End:   defaultTime.Add(3 * time.Hour),
				},
			},
			forecasts: []Forecast{
				{
					Time:             defaultTime,
					WeatherVariables: noFogVariables,
				},
				{
					Time:             defaultTime.Add(1 * time.Hour),
					WeatherVariables: noFogVariables,
				},
				{
					Time:             defaultTime.Add(2 * time.Hour),
					WeatherVariables: noFogVariables,
				},
			},
			expected: AlertChange{
				Type:  Revoked,
				Alert: nil,
			},
		},
		{
			name: "fog - no current alert",
			monitor: Monitor{
				ID:       defaultUUID,
				UserID:   defaultUUID,
				IsActive: true,
				Location: defaultLocation,
			},
			forecasts: []Forecast{
				{
					Time:             defaultTime,
					WeatherVariables: fogVariables,
				},
				{
					Time:             defaultTime.Add(1 * time.Hour),
					WeatherVariables: fogVariables,
				},
				{
					Time:             defaultTime.Add(2 * time.Hour),
					WeatherVariables: noFogVariables,
				},
			},
			expected: AlertChange{
				Type: New,
				Alert: &Alert{
					Start: defaultTime,
					End:   defaultTime.Add(1 * time.Hour),
				},
			},
		},
		{
			name: "fog - existing alert - no change",
			monitor: Monitor{
				ID:       defaultUUID,
				UserID:   defaultUUID,
				IsActive: true,
				Location: defaultLocation,
				ActiveAlert: &Alert{
					Start: defaultTime,
					End:   defaultTime.Add(1 * time.Hour),
				},
			},
			forecasts: []Forecast{
				{
					Time:             defaultTime,
					WeatherVariables: fogVariables,
				},
				{
					Time:             defaultTime.Add(1 * time.Hour),
					WeatherVariables: fogVariables,
				},
				{
					Time:             defaultTime.Add(2 * time.Hour),
					WeatherVariables: noFogVariables,
				},
			},
			expected: AlertChange{
				Type:  Unchanged,
				Alert: &Alert{Start: defaultTime, End: defaultTime.Add(1 * time.Hour)},
			},
		},
		{
			name: "fog - existing alert - changed",
			monitor: Monitor{
				ID:       defaultUUID,
				UserID:   defaultUUID,
				IsActive: true,
				Location: defaultLocation,
				ActiveAlert: &Alert{
					Start: defaultTime,
					End:   defaultTime.Add(1 * time.Hour),
				},
			},
			forecasts: []Forecast{
				{
					Time:             defaultTime,
					WeatherVariables: fogVariables,
				},
				{
					Time:             defaultTime.Add(1 * time.Hour),
					WeatherVariables: fogVariables,
				},
				{
					Time:             defaultTime.Add(2 * time.Hour),
					WeatherVariables: fogVariables,
				},
			},
			expected: AlertChange{
				Type: Changed,
				Alert: &Alert{
					Start: defaultTime,
					End:   defaultTime.Add(2 * time.Hour),
				},
			},
		},
		{
			name: "fog - existing alert - revoked",
			monitor: Monitor{
				ID:       defaultUUID,
				UserID:   defaultUUID,
				IsActive: true,
				Location: defaultLocation,
				ActiveAlert: &Alert{
					Start: defaultTime,
					End:   defaultTime.Add(1 * time.Hour),
				},
			},
			forecasts: []Forecast{
				{
					Time:             defaultTime,
					WeatherVariables: noFogVariables,
				},
				{
					Time:             defaultTime.Add(1 * time.Hour),
					WeatherVariables: noFogVariables,
				},
				{
					Time:             defaultTime.Add(2 * time.Hour),
					WeatherVariables: noFogVariables,
				},
			},
			expected: AlertChange{
				Type:  Revoked,
				Alert: nil,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.monitor.ReconcileAlert(tc.forecasts)
			if diff := cmp.Diff(tc.expected, got); diff != "" {
				t.Errorf("ReconcileAlert() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
