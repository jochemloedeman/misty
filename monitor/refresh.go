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

const (
	fogDewPointSpread    = 2.5  // max °C difference between temperature and dew point
	fogVisibilityLimit   = 1000 // max visibility in meters
	fogHumidityThreshold = 95   // min relative humidity in percent
	fogWindSpeedLimit    = 10   // max wind speed in m/s
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
	Update(ctx context.Context, m Monitor) (Monitor, error)
}

type Forecaster interface {
	Forecast(
		ctx context.Context,
		location Location,
		horizon ForecastHorizon,
	) ([]Forecast, error)
}

type WeatherVariables struct {
	Temperature      float64 `unit:"°C"`
	DewPoint         float64 `unit:"°C"`
	RelativeHumidity float64 `unit:"%"`
	WindSpeed        float64 `unit:"m/s"`
	Visibility       float64 `unit:"m"`
}

func (w WeatherVariables) IsFogLikely() bool {
	dewPointClose := (w.Temperature - w.DewPoint) < fogDewPointSpread
	poorVisibility := w.Visibility < fogVisibilityLimit
	highHumidity := w.RelativeHumidity > fogHumidityThreshold
	calmWinds := w.WindSpeed < fogWindSpeedLimit
	return dewPointClose && poorVisibility && highHumidity && calmWinds
}

type Forecast struct {
	Time time.Time
	WeatherVariables
}

type ForecastStore interface {
	Save(
		ctx context.Context,
		monitorID uuid.UUID,
		forecasts []Forecast,
	) ([]Forecast, error)
}

type NotificationOutbox interface {
	Create(
		ctx context.Context,
		notif notifications.Notification,
	) (notifications.Notification, error)
}

type RunAtomically func(ctx context.Context, fn func(s AtomicStores) error) error

type Refresher struct {
	clock        Clock
	forecaster   Forecaster
	monitorStore MonitorStore
	runAtom      RunAtomically
}

func NewRefresher(
	forecaster Forecaster,
	monitorStore MonitorStore,
	runAtom RunAtomically,
	clock Clock,
) *Refresher {
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

func (r *Refresher) RefreshAll(
	ctx context.Context,
	horizon ForecastHorizon,
) error {
	monitors, err := r.monitorStore.ListAllActive(ctx)
	if err != nil {
		return fmt.Errorf("list active monitors: %w", err)
	}

	slog.Info("refresh started", "monitor_count", len(monitors))

	for i := range monitors {
		err := r.refresh(ctx, monitors[i], horizon)
		if err == nil {
			continue
		}
		if _, ok := errors.AsType[*Transient](err); ok {
			slog.Warn(
				"transient error refreshing monitor",
				"monitor_id",
				monitors[i].ID,
				"error",
				err,
			)
			continue
		}
		return fmt.Errorf("refresh monitor %s: %w", monitors[i].ID, err)
	}

	slog.Info("refresh completed", "monitor_count", len(monitors))
	return nil
}

func (r *Refresher) refresh(
	ctx context.Context,
	monitor Monitor,
	horizon ForecastHorizon,
) error {
	now := r.clock.Now()
	forecasts, err := r.forecaster.Forecast(ctx, monitor.Location, horizon)
	if err != nil {
		return fmt.Errorf("forecast: %w", err)
	}

	monitor, alertChange := monitor.ReconcileAlert(now, forecasts)

	slog.Info("alert reconciled",
		"monitor_id", monitor.ID,
		"location", monitor.Location.Name,
		"change_type", alertChange.Type,
	)

	return r.runAtom(ctx, func(s AtomicStores) error {
		return persist(ctx, s, monitor, forecasts, alertChange)
	})
}

func persist(
	ctx context.Context,
	s AtomicStores,
	monitor Monitor,
	forecasts []Forecast,
	ac AlertChange,
) error {
	if _, err := s.ForecastStore.Save(ctx, monitor.ID, forecasts); err != nil {
		return fmt.Errorf("save forecasts: %w", err)
	}

	if ac.NeedsNotification() {
		msg := fogAlertMessage(monitor, ac)
		notif := notifications.NewNotification(monitor.UserID, msg)
		if _, err := s.Outbox.Create(ctx, notif); err != nil {
			return fmt.Errorf("create notification: %w", err)
		}
		slog.Info(
			"notification queued",
			"monitor_id",
			monitor.ID,
			"user_id",
			monitor.UserID,
			"message",
			msg,
		)
	}

	if ac.NeedsSave() {
		if _, err := s.MonitorStore.Update(ctx, monitor); err != nil {
			return fmt.Errorf("update alert: %w", err)
		}
		slog.Debug("monitor updated", "monitor_id", monitor.ID)
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
