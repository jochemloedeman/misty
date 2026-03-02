package monitor

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type MonitorStore interface {
	ListActive(ctx context.Context) ([]Monitor, error)
	Save(ctx context.Context, monitor Monitor) (Monitor, error)
}

type WeatherForecaster interface {
	Forecast(ctx context.Context, location Location, timeRange TimeRange) ([]Forecast, error)
}

type TimeRange struct {
	Start time.Time
	End   time.Time
}

type WeatherVariables struct {
	Temperature      float64 `unit:"°C"`
	DewPoint         float64 `unit:"°C"`
	RelativeHumidity float64 `unit:"%"`
	WindSpeed        float64 `unit:"m/s"`
	Visibility       int     `unit:"m"`
}

func (w WeatherVariables) FogIsLikely() bool {
	dewPointClose := (w.Temperature - w.DewPoint) < 2.5
	poorVisibility := w.Visibility < 1000
	highHumidity := w.RelativeHumidity > 95
	calmWinds := w.WindSpeed < 10
	return dewPointClose && poorVisibility && highHumidity && calmWinds
}

type Forecast struct {
	Time time.Time
	WeatherVariables
}

type ForecastStore interface {
	Save(ctx context.Context, monitorID uuid.UUID, forecasts []Forecast) error
}

type NotificationOutbox interface {
	AddNew(ctx context.Context, not Notification) error
}

type NotificationStatus string

const (
	Pending NotificationStatus = "pending"
	Sent    NotificationStatus = "sent"
)

type Notification struct {
	Message string
	Status  NotificationStatus
}

const horizon = 12 * time.Hour

type Refresher struct {
	Forecaster    WeatherForecaster
	MonitorStore  MonitorStore
	ForecastStore ForecastStore
	Outbox        NotificationOutbox
}

func (r *Refresher) RefreshMonitors(ctx context.Context) error {
	activeMonitors, err := r.MonitorStore.ListActive(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch monitors: %w", err)
	}

	now := time.Now()
	timeRange := TimeRange{
		Start: now,
		End:   now.Add(horizon),
	}

	for i := range activeMonitors {
		monitor := &activeMonitors[i]
		forecasts, err := r.Forecaster.Forecast(ctx, monitor.Location, timeRange)
		if err != nil {
			return fmt.Errorf("failed to forecast: %w", err)
		}

		err = r.ForecastStore.Save(ctx, monitor.ID, forecasts)
		if err != nil {
			return fmt.Errorf("failed to store forecasts: %w", err)
		}

		alertChange := monitor.EvaluateAlert(forecasts)

		monitor.Alert = alertChange.Alert
		if alertChange.NeedsNotification() {
			err = r.Outbox.AddNew(ctx, Notification{})
			if err != nil {
				return fmt.Errorf("failed to store notification: %w", err)
			}
		}
		if alertChange.NeedsSave() {
			if _, err := r.MonitorStore.Save(ctx, *monitor); err != nil {
				return fmt.Errorf("failed to store monitor: %w", err)
			}
		}

	}

	return nil

}
