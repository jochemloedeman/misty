package monitor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jochemloedeman/misty/notification"
)

const (
	fogDewPointSpread    = 2.5  // max °C difference between temperature and dew point
	fogVisibilityLimit   = 1000 // max visibility in meters
	fogHumidityThreshold = 95   // min relative humidity in percent
	fogWindSpeedLimit    = 10   // max wind speed in m/s

	wmoFog     = 45
	wmoRimeFog = 48
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
	WindSpeed        float64 `unit:"km/h"`
	Visibility       float64 `unit:"m"`
	WeatherCode      int     `unit:"wmo code"`
}

func (w WeatherVariables) IsFogLikely() bool {
	if w.WeatherCode == wmoFog || w.WeatherCode == wmoRimeFog {
		return true
	}
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
		notif notification.Fog,
	) (notification.Fog, error)
}

type RunAtomically func(ctx context.Context, fn func(s AtomicStores) error) error

type Refresher struct {
	clock        Clock
	forecaster   Forecaster
	monitorStore MonitorStore
	runAtom      RunAtomically
	refreshC     chan Monitor
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
		refreshC:     make(chan Monitor, 8),
	}
}

func (r *Refresher) RefreshC() <-chan Monitor {
	return r.refreshC
}

func (r *Refresher) RequestRefresh(ctx context.Context, m Monitor) {
	select {
	case r.refreshC <- m:
		slog.DebugContext(
			ctx,
			"immediate refresh requested",
			"monitor_id",
			m.ID,
		)
	default:
		slog.WarnContext(
			ctx,
			"immediate refresh dropped, buffer full",
			"monitor_id",
			m.ID,
		)
	}
}

func (r *Refresher) RefreshOne(
	ctx context.Context,
	m Monitor,
	horizon ForecastHorizon,
) error {
	return r.refresh(ctx, m, horizon)
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

	slog.InfoContext(ctx, "refresh started", "monitor_count", len(monitors))

	for i := range monitors {
		err := r.refresh(ctx, monitors[i], horizon)
		if err == nil {
			continue
		}
		if _, ok := errors.AsType[*Transient](err); ok {
			slog.WarnContext(
				ctx,
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

	slog.InfoContext(ctx, "refresh completed", "monitor_count", len(monitors))
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

	monitor, change := monitor.ReconcileRiskWindow(
		now,
		forecasts,
		horizon.Interval,
	)

	slog.InfoContext(ctx, "alert reconciled",
		"monitor_id", monitor.ID,
		"location", monitor.Location.Name,
		"change_type", change.Type,
	)

	return r.runAtom(ctx, func(s AtomicStores) error {
		return persist(ctx, s, monitor, forecasts, change)
	})
}

func persist(
	ctx context.Context,
	s AtomicStores,
	monitor Monitor,
	forecasts []Forecast,
	change RiskWindowChange,
) error {
	if _, err := s.ForecastStore.Save(ctx, monitor.ID, forecasts); err != nil {
		return fmt.Errorf("save forecasts: %w", err)
	}

	if change.NeedsNotification() {
		msg := fogAlertMessage(monitor)
		notif := notification.New(
			monitor.UserID,
			msg,
			monitor.Location.Name,
			change.RiskWindow.Start,
			change.RiskWindow.End,
		)
		if _, err := s.Outbox.Create(ctx, notif); err != nil {
			return fmt.Errorf("create notification: %w", err)
		}
		slog.InfoContext(
			ctx,
			"notification queued",
			"monitor_id",
			monitor.ID,
			"user_id",
			monitor.UserID,
			"message",
			msg,
		)
	}

	if change.NeedsSave() {
		if _, err := s.MonitorStore.Update(ctx, monitor); err != nil {
			return fmt.Errorf("update alert: %w", err)
		}
		slog.DebugContext(ctx, "monitor updated", "monitor_id", monitor.ID)
	}

	return nil
}

func fogAlertMessage(m Monitor) string {
	return fmt.Sprintf("Fog is forecast for %s", m.Location.Name)
}
