package monitor

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type MonitorStore interface {
	ListActive(ctx context.Context) ([]Monitor, error)
	UpdateAlert(ctx context.Context, monitorID uuid.UUID, alert *Alert) (Monitor, error)
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
	ForecastAt time.Time
	WeatherVariables
}

type ForecastStore interface {
	Save(ctx context.Context, monitorID uuid.UUID, forecasts []Forecast) ([]Forecast, error)
}

type NotificationOutbox interface {
	Create(ctx context.Context, notif Notification) (Notification, error)
}

type Notification struct {
	ID          uuid.UUID
	RecipientID uuid.UUID
	Message     string
	SentAt      time.Time
}

func NewNotification(recipientID uuid.UUID, message string) Notification {
	return Notification{
		ID:          uuid.New(),
		RecipientID: recipientID,
		Message:     message,
	}
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

		forecasts, err = r.ForecastStore.Save(ctx, monitor.ID, forecasts)
		if err != nil {
			return fmt.Errorf("failed to store forecasts: %w", err)
		}

		alertChange := monitor.EvaluateAlert(forecasts)

		if alertChange.NeedsNotification() {
			notif := NewNotification(
				monitor.UserID,
				fmt.Sprintf(
					"Fog alert for %s from %s to %s",
					monitor.Location.Name,
					alertChange.Alert.Start.Format(time.Kitchen),
					alertChange.Alert.End.Format(time.Kitchen),
				),
			)
			_, err = r.Outbox.Create(ctx, notif)
			if err != nil {
				return fmt.Errorf("failed to store notification: %w", err)
			}
		}
		if alertChange.NeedsSave() {
			if _, err := r.MonitorStore.UpdateAlert(ctx, monitor.ID, alertChange.Alert); err != nil {
				return fmt.Errorf("failed to store monitor: %w", err)
			}
		}
	}

	return nil

}
