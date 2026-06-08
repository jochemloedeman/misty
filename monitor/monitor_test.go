package monitor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
)

var (
	defaultUUID     = uuid.MustParse("00000000-0000-0000-0000-000000000000")
	defaultLocation = Location{
		Name: "Test Location",
		Lat:  0.0,
		Lon:  0.0,
	}
)

var (
	defaultTime    = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	noFogVariables = WeatherVariables{
		Temperature:      20,
		DewPoint:         10,
		RelativeHumidity: 50,
		WindSpeed:        5,
		Visibility:       10000,
		WeatherCode:      0,
	}
)

var fogVariables = WeatherVariables{
	Temperature:      10,
	DewPoint:         9,
	RelativeHumidity: 98,
	WindSpeed:        2,
	Visibility:       500,
	WeatherCode:      45,
}

var errStub = errors.New("db down")

type stubValidator struct {
	count     int
	countErr  error
	exists    bool
	existsErr error
}

func (s stubValidator) CountByUser(context.Context, uuid.UUID) (int, error) {
	return s.count, s.countErr
}

func (s stubValidator) LocationExistsByUser(
	context.Context,
	uuid.UUID,
	float64,
	float64,
) (bool, error) {
	return s.exists, s.existsErr
}

func TestNewMonitor(t *testing.T) {
	ctx := t.Context()
	uid := uuid.New()
	loc := Location{Name: "Test", Lat: 52.0, Lon: 5.0}

	tests := []struct {
		name      string
		validator stubValidator
		limit     int
		wantErr   error
	}{
		{
			name:      "below limit",
			validator: stubValidator{count: 2},
			limit:     5,
		},
		{
			name:      "at limit",
			validator: stubValidator{count: 5},
			limit:     5,
			wantErr:   ErrLimitReached,
		},
		{
			name:      "above limit",
			validator: stubValidator{count: 7},
			limit:     5,
			wantErr:   ErrLimitReached,
		},
		{
			name:      "counter error propagated",
			validator: stubValidator{countErr: errStub},
			limit:     5,
			wantErr:   errStub,
		},
		{
			name:      "duplicate location",
			validator: stubValidator{count: 2, exists: true},
			limit:     5,
			wantErr:   ErrDuplicateLocation,
		},
		{
			name:      "location check error propagated",
			validator: stubValidator{count: 2, existsErr: errStub},
			limit:     5,
			wantErr:   errStub,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewMonitor(ctx, tt.validator, uid, loc, tt.limit)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("NewMonitor() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("NewMonitor() unexpected error: %v", err)
			}
		})
	}
}

func TestReconcileRiskWindow(t *testing.T) {
	testCases := []struct {
		name      string
		monitor   Monitor
		forecasts []Forecast
		expected  RiskWindowChange
	}{
		{
			name: "clear - no current risk window",
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
			expected: RiskWindowChange{
				Type:       Unchanged,
				RiskWindow: nil,
			},
		},
		{
			name: "clear - existing risk window",
			monitor: Monitor{
				ID:       defaultUUID,
				UserID:   defaultUUID,
				IsActive: true,
				Location: defaultLocation,
				RiskWindow: &RiskWindow{
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
			expected: RiskWindowChange{
				Type:       Revoked,
				RiskWindow: nil,
			},
		},
		{
			name: "fog - no current risk window",
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
			expected: RiskWindowChange{
				Type: New,
				RiskWindow: &RiskWindow{
					Start: defaultTime,
					End:   defaultTime.Add(2 * time.Hour),
				},
			},
		},
		{
			name: "fog - existing risk window - no change",
			monitor: Monitor{
				ID:       defaultUUID,
				UserID:   defaultUUID,
				IsActive: true,
				Location: defaultLocation,
				RiskWindow: &RiskWindow{
					Start: defaultTime,
					End:   defaultTime.Add(2 * time.Hour),
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
			expected: RiskWindowChange{
				Type: Unchanged,
				RiskWindow: &RiskWindow{
					Start: defaultTime,
					End:   defaultTime.Add(2 * time.Hour),
				},
			},
		},
		{
			name: "fog - existing risk window - changed",
			monitor: Monitor{
				ID:       defaultUUID,
				UserID:   defaultUUID,
				IsActive: true,
				Location: defaultLocation,
				RiskWindow: &RiskWindow{
					Start: defaultTime,
					End:   defaultTime.Add(2 * time.Hour),
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
			expected: RiskWindowChange{
				Type: Changed,
				RiskWindow: &RiskWindow{
					Start: defaultTime,
					End:   defaultTime.Add(3 * time.Hour),
				},
			},
		},
		{
			name: "fog - existing risk window - revoked",
			monitor: Monitor{
				ID:       defaultUUID,
				UserID:   defaultUUID,
				IsActive: true,
				Location: defaultLocation,
				RiskWindow: &RiskWindow{
					Start: defaultTime,
					End:   defaultTime.Add(2 * time.Hour),
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
			expected: RiskWindowChange{
				Type:       Revoked,
				RiskWindow: nil,
			},
		},
		{
			name: "no fog - existing risk window expired",
			monitor: Monitor{
				ID:       defaultUUID,
				UserID:   defaultUUID,
				IsActive: true,
				Location: defaultLocation,
				RiskWindow: &RiskWindow{
					Start: defaultTime.Add(-2 * time.Hour),
					End:   defaultTime.Add(-1 * time.Hour),
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
			expected: RiskWindowChange{
				Type:       Revoked,
				RiskWindow: nil,
			},
		},
		{
			name: "fog - existing risk window expired",
			monitor: Monitor{
				ID:       defaultUUID,
				UserID:   defaultUUID,
				IsActive: true,
				Location: defaultLocation,
				RiskWindow: &RiskWindow{
					Start: defaultTime.Add(-2 * time.Hour),
					End:   defaultTime.Add(-1 * time.Hour),
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
			expected: RiskWindowChange{
				Type: New,
				RiskWindow: &RiskWindow{
					Start: defaultTime,
					End:   defaultTime.Add(2 * time.Hour),
				},
			},
		},
		{
			name: "fog - existing risk window - non-overlapping new risk window after",
			monitor: Monitor{
				ID:       defaultUUID,
				UserID:   defaultUUID,
				IsActive: true,
				Location: defaultLocation,
				RiskWindow: &RiskWindow{
					Start: defaultTime,
					End:   defaultTime.Add(2 * time.Hour),
				},
			},
			forecasts: []Forecast{
				{
					Time:             defaultTime.Add(3 * time.Hour),
					WeatherVariables: fogVariables,
				},
				{
					Time:             defaultTime.Add(4 * time.Hour),
					WeatherVariables: fogVariables,
				},
			},
			expected: RiskWindowChange{
				Type: New,
				RiskWindow: &RiskWindow{
					Start: defaultTime.Add(3 * time.Hour),
					End:   defaultTime.Add(5 * time.Hour),
				},
			},
		},
		{
			name: "fog - existing risk window - non-overlapping new risk window before",
			monitor: Monitor{
				ID:       defaultUUID,
				UserID:   defaultUUID,
				IsActive: true,
				Location: defaultLocation,
				RiskWindow: &RiskWindow{
					Start: defaultTime.Add(3 * time.Hour),
					End:   defaultTime.Add(5 * time.Hour),
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
			},
			expected: RiskWindowChange{
				Type: New,
				RiskWindow: &RiskWindow{
					Start: defaultTime,
					End:   defaultTime.Add(2 * time.Hour),
				},
			},
		},
		{
			name: "fog detected by weather code only",
			monitor: Monitor{
				ID:       defaultUUID,
				UserID:   defaultUUID,
				IsActive: true,
				Location: defaultLocation,
			},
			forecasts: []Forecast{
				{
					Time: defaultTime,
					WeatherVariables: WeatherVariables{
						Temperature:      20,
						DewPoint:         10,
						RelativeHumidity: 50,
						WindSpeed:        5,
						Visibility:       24140,
						WeatherCode:      45,
					},
				},
				{
					Time: defaultTime.Add(1 * time.Hour),
					WeatherVariables: WeatherVariables{
						Temperature:      20,
						DewPoint:         10,
						RelativeHumidity: 50,
						WindSpeed:        5,
						Visibility:       24140,
						WeatherCode:      48,
					},
				},
				{
					Time:             defaultTime.Add(2 * time.Hour),
					WeatherVariables: noFogVariables,
				},
			},
			expected: RiskWindowChange{
				Type: New,
				RiskWindow: &RiskWindow{
					Start: defaultTime,
					End:   defaultTime.Add(2 * time.Hour),
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotMonitor, gotChange := tc.monitor.ReconcileRiskWindow(
				defaultTime,
				tc.forecasts,
				time.Hour,
			)
			if diff := cmp.Diff(tc.expected, gotChange); diff != "" {
				t.Errorf(
					"ReconcileRiskWindow() RiskWindowChange mismatch (-want +got):\n%s",
					diff,
				)
			}
			if diff := cmp.Diff(
				tc.expected.RiskWindow,
				gotMonitor.RiskWindow,
			); diff != "" {
				t.Errorf(
					"ReconcileRiskWindow() Monitor.RiskWindow mismatch (-want +got):\n%s",
					diff,
				)
			}
		})
	}
}
