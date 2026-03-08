package monitor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jochemloedeman/misty/notifications"
)

type Transient struct {
	Err error
}

func (t *Transient) Error() string {
	return t.Err.Error()
}

func (t *Transient) Unwrap() error {
	return t.Err
}

type Clock interface {
	Now() time.Time
}

type MonitorStore interface {
	ListAllActive(ctx context.Context) ([]Monitor, error)
	UpdateAlert(ctx context.Context, monitorID uuid.UUID, alert *Alert) (Monitor, error)
}

type Forecaster interface {
	Forecast(ctx context.Context, location Location, horizon TimeHorizon) ([]Forecast, error)
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
	Create(ctx context.Context, notif notifications.Notification) (notifications.Notification, error)
}

type RunAtomically func(ctx context.Context, fn func(s AtomicStores) error) error

type Refresher struct {
	clock        Clock
	forecaster   Forecaster
	monitorStore MonitorStore
	runAtom      RunAtomically
}

func NewRefresher(forecaster Forecaster, monitorStore MonitorStore, runAtom RunAtomically, clock Clock) *Refresher {
	return &Refresher{
		forecaster:   forecaster,
		monitorStore: monitorStore,
		runAtom:      runAtom,
		clock:        clock,
	}
}

type AtomicStores struct {
	MonitorStore  MonitorStore
	ForecastStore ForecastStore
	Outbox        NotificationOutbox
}

func (r *Refresher) RefreshAll(ctx context.Context, horizon TimeHorizon) error {
	monitors, err := r.monitorStore.ListAllActive(ctx)
	if err != nil {
		return fmt.Errorf("list active monitors: %w", err)
	}

	for i := range monitors {
		err := r.refresh(ctx, monitors[i], horizon)
		if err == nil {
			continue
		}
		if _, ok := errors.AsType[*Transient](err); ok {
			slog.Warn("transient error refreshing monitor", "monitor_id", monitors[i].ID, "error", err)
			continue
		}
		return fmt.Errorf("refresh monitor %s: %w", monitors[i].ID, err)
	}
	return nil
}

func (r *Refresher) refresh(ctx context.Context, monitor Monitor, horizon TimeHorizon) error {
	now := r.clock.Now()
	forecasts, err := r.forecaster.Forecast(ctx, monitor.Location, horizon)

	if err != nil {
		return fmt.Errorf("forecast: %w", err)
	}

	alertChange := monitor.ReconcileAlert(now, forecasts)

	return r.runAtom(ctx, func(s AtomicStores) error {
		return persist(ctx, s, monitor, forecasts, alertChange)
	})
}

func persist(ctx context.Context, s AtomicStores, monitor Monitor, forecasts []Forecast, ac AlertChange) error {
	if _, err := s.ForecastStore.Save(ctx, monitor.ID, forecasts); err != nil {
		return fmt.Errorf("save forecasts: %w", err)
	}

	if ac.NeedsNotification() {
		notif := notifications.NewNotification(monitor.UserID, fogAlertMessage(monitor, ac))
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
		ac.Alert.Start.Format("15:04"),
		ac.Alert.End.Format("15:04"),
	)
}
