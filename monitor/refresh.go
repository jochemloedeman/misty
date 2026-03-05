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

type Forecaster interface {
	Forecast(ctx context.Context, location Location, horizon ForecastHorizon) ([]Forecast, error)
}

type ForecastHorizon struct {
	Granularity time.Duration
	Steps       int
}

type WeatherVariables struct {
	Temperature      float64 `unit:"°C"`
	DewPoint         float64 `unit:"°C"`
	RelativeHumidity float64 `unit:"%"`
	WindSpeed        float64 `unit:"m/s"`
	Visibility       int     `unit:"m"`
}

func (w WeatherVariables) IsFogLikely() bool {
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

type RunAtomically func(ctx context.Context, fn func(s AtomicStores) error) error

type Refresher struct {
	forecaster   Forecaster
	monitorStore MonitorStore
	runAtom      RunAtomically
}

func NewRefresher(forecaster Forecaster, monitorStore MonitorStore, runAtom RunAtomically) *Refresher {
	return &Refresher{
		forecaster:   forecaster,
		monitorStore: monitorStore,
		runAtom:      runAtom,
	}
}

type AtomicStores struct {
	MonitorStore  MonitorStore
	ForecastStore ForecastStore
	Outbox        NotificationOutbox
}

func (r *Refresher) RefreshAll(ctx context.Context, horizon ForecastHorizon) error {
	monitors, err := r.monitorStore.ListActive(ctx)
	if err != nil {
		return fmt.Errorf("list active monitors: %w", err)
	}

	for i := range monitors {
		if err := r.refresh(ctx, monitors[i], horizon); err != nil {
			return fmt.Errorf("refresh monitor %s: %w", monitors[i].ID, err)
		}
	}
	return nil
}

func (r *Refresher) refresh(ctx context.Context, monitor Monitor, horizon ForecastHorizon) error {
	forecasts, err := r.forecaster.Forecast(ctx, monitor.Location, horizon)
	if err != nil {
		return fmt.Errorf("forecast: %w", err)
	}

	alertChange := monitor.ReconcileAlert(forecasts)

	return r.runAtom(ctx, func(s AtomicStores) error {
		return persist(ctx, s, monitor, forecasts, alertChange)
	})
}

func persist(ctx context.Context, s AtomicStores, monitor Monitor, forecasts []Forecast, ac AlertChange) error {
	if _, err := s.ForecastStore.Save(ctx, monitor.ID, forecasts); err != nil {
		return fmt.Errorf("save forecasts: %w", err)
	}

	if ac.NeedsNotification() {
		notif := NewNotification(monitor.UserID, fogAlertMessage(monitor, ac))
		if _, err := s.Outbox.Create(ctx, notif); err != nil {
			return fmt.Errorf("create notification: %w", err)
		}
	}

	if ac.NeedsSave() {
		if _, err := s.MonitorStore.UpdateAlert(ctx, monitor.ID, ac.Alert); err != nil {
			return fmt.Errorf("update alert: %w", err)
		}
	}

	return nil
}

func fogAlertMessage(m Monitor, ac AlertChange) string {
	return fmt.Sprintf("Fog alert for %s from %s to %s",
		m.Location.Name,
		ac.Alert.Start.Format(time.Kitchen),
		ac.Alert.End.Format(time.Kitchen),
	)
}
