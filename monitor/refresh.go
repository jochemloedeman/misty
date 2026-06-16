package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jochemloedeman/misty/notification"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var meter = otel.Meter("github.com/jochemloedeman/misty/monitor")

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

type metrics struct {
	reconciled metric.Int64Counter
}

func newMetrics() (*metrics, error) {
	reconciled, err := meter.Int64Counter(
		"monitors.reconciled",
		metric.WithDescription("Number of monitor risk-window reconciliations by change type"),
		metric.WithUnit("{monitor}"),
	)
	return &metrics{reconciled: reconciled}, err
}

type Refresher struct {
	clock      Clock
	forecaster Forecaster
	runAtom    RunAtomically
	metrics    *metrics
}

func NewRefresher(
	forecaster Forecaster,
	runAtom RunAtomically,
	clock Clock,
) (*Refresher, error) {
	m, err := newMetrics()
	if err != nil {
		return nil, fmt.Errorf("create monitor metrics: %w", err)
	}
	return &Refresher{
		forecaster: forecaster,
		runAtom:    runAtom,
		clock:      clock,
		metrics:    m,
	}, nil
}

type AtomicStores struct {
	MonitorStore  MonitorStore
	ForecastStore ForecastStore
	Outbox        NotificationOutbox
}

func (r *Refresher) Refresh(
	ctx context.Context,
	monitor Monitor,
	horizon ForecastHorizon,
) error {
	now := r.clock.Now()
	forecasts, err := r.forecaster.Forecast(ctx, monitor.Location, horizon)
	if err != nil {
		return fmt.Errorf("forecast: %w", err)
	}

	monitor, change := monitor.ReconcileRiskWindow(now, forecasts, horizon.Interval)

	slog.InfoContext(ctx, "alert reconciled",
		"monitor_id", monitor.ID,
		"location", monitor.Location.Name,
		"change_type", change.Type,
	)

	r.metrics.reconciled.Add(ctx, 1, metric.WithAttributes(attribute.String("change_type", change.Type.String())))

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
		slog.InfoContext(ctx, "notification queued",
			"monitor_id", monitor.ID,
			"user_id", monitor.UserID,
			"message", msg,
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
