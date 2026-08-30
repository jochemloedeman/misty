package monitor

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func forecastAt(ts time.Time, temperature float64) Forecast {
	vars := noFogVariables
	vars.Temperature = temperature
	return Forecast{Time: ts, WeatherVariables: vars}
}

func TestDedupeByTime(t *testing.T) {
	plusTwo := time.FixedZone("UTC+2", 2*60*60)

	tests := []struct {
		name      string
		forecasts []Forecast
		want      []Forecast
	}{
		{
			name: "no duplicates",
			forecasts: []Forecast{
				forecastAt(defaultTime, 1),
				forecastAt(defaultTime.Add(time.Hour), 2),
			},
			want: []Forecast{
				forecastAt(defaultTime, 1),
				forecastAt(defaultTime.Add(time.Hour), 2),
			},
		},
		{
			name: "unsorted input is sorted by time",
			forecasts: []Forecast{
				forecastAt(defaultTime.Add(2*time.Hour), 3),
				forecastAt(defaultTime, 1),
				forecastAt(defaultTime.Add(time.Hour), 2),
			},
			want: []Forecast{
				forecastAt(defaultTime, 1),
				forecastAt(defaultTime.Add(time.Hour), 2),
				forecastAt(defaultTime.Add(2*time.Hour), 3),
			},
		},
		{
			name: "duplicate timestamp keeps last",
			forecasts: []Forecast{
				forecastAt(defaultTime, 1),
				forecastAt(defaultTime, 2),
				forecastAt(defaultTime, 3),
			},
			want: []Forecast{forecastAt(defaultTime, 3)},
		},
		{
			name: "same instant in another location collapses",
			forecasts: []Forecast{
				forecastAt(defaultTime, 1),
				forecastAt(defaultTime.In(plusTwo), 2),
			},
			want: []Forecast{forecastAt(defaultTime, 2)},
		},
		{
			name:      "single element",
			forecasts: []Forecast{forecastAt(defaultTime, 1)},
			want:      []Forecast{forecastAt(defaultTime, 1)},
		},
		{
			name:      "empty",
			forecasts: []Forecast{},
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dedupeByTime(tt.forecasts)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("dedupeByTime() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
