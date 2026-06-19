package monitor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jochemloedeman/misty/notification"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	meter              = otel.Meter("github.com/jochemloedeman/misty/monitor")
	tracer             = otel.Tracer("github.com/jochemloedeman/misty/monitor")
	durationBoundaries = []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5}
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

type metrics struct {
	reconciled        metric.Int64Counter
	refreshDuration   metric.Float64Histogram
	monitorsRefreshed metric.Int64Gauge
	usersRefreshed    metric.Int64Gauge
}

func newMetrics() (*metrics, error) {
	var err, e error
	reconciled, e := meter.Int64Counter(
		"monitors.reconciled",
		metric.WithDescription("Number of monitor risk-window reconciliations by change type"),
		metric.WithUnit("{monitor}"),
	)
	if e != nil {
		err = errors.Join(err, e)
	}
	refreshDuration, e := meter.Float64Histogram(
		"refresh.duration",
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBoundaries...),
	)
	if e != nil {
		err = errors.Join(err, e)
	}
	monitorsRefreshed, e := meter.Int64Gauge("monitors.refreshed", metric.WithUnit("{monitor}"))
	if e != nil {
		err = errors.Join(err, e)
	}
	usersRefreshed, e := meter.Int64Gauge("users.refreshed", metric.WithUnit("{user}"))
	if e != nil {
		err = errors.Join(err, e)
	}
	return &metrics{
		reconciled:        reconciled,
		refreshDuration:   refreshDuration,
		monitorsRefreshed: monitorsRefreshed,
		usersRefreshed:    usersRefreshed,
	}, err
}

type Refresher struct {
	clock      Clock
	forecaster Forecaster
	runAtom    RunAtomically
	store      MonitorStore
	horizon    ForecastHorizon
	metrics    *metrics
}

func NewRefresher(
	forecaster Forecaster,
	runAtom RunAtomically,
	clock Clock,
	store MonitorStore,
	horizon ForecastHorizon,
) (*Refresher, error) {
	m, err := newMetrics()
	if err != nil {
		return nil, fmt.Errorf("create monitor metrics: %w", err)
	}
	return &Refresher{
		forecaster: forecaster,
		runAtom:    runAtom,
		clock:      clock,
		store:      store,
		horizon:    horizon,
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
) (queued *notification.Queued, err error) {
	ctx, span := tracer.Start(ctx, "refresh", trace.WithAttributes(
		attribute.String("monitor.id", monitor.ID.String()),
		attribute.String("monitor.location", monitor.Location.Name),
	))
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	now := r.clock.Now()
	forecasts, err := r.forecaster.Forecast(ctx, monitor.Location, r.horizon)
	if err != nil {
		return nil, fmt.Errorf("forecast: %w", err)
	}

	monitor, change := monitor.ReconcileRiskWindow(now, forecasts, r.horizon.Interval)

	slog.InfoContext(ctx, "alert reconciled",
		"monitor_id", monitor.ID,
		"location", monitor.Location.Name,
		"change_type", change.Type,
	)

	r.metrics.reconciled.Add(ctx, 1, metric.WithAttributes(attribute.String("change_type", change.Type.String())))

	if err := r.runAtom(ctx, func(s AtomicStores) error {
		var e error
		queued, e = persist(ctx, s, monitor, forecasts, change)
		return e
	}); err != nil {
		return nil, err
	}

	return queued, nil
}

func (r *Refresher) RefreshAll(ctx context.Context) (err error) {
	ctx, span := tracer.Start(ctx, "refresh.all")
	start := r.clock.Now()
	defer func() {
		var attrs []attribute.KeyValue
		if err != nil {
			attrs = append(attrs, semconv.ErrorTypeKey.String("refresh_failed"))
		}
		r.metrics.refreshDuration.Record(ctx, r.clock.Now().Sub(start).Seconds(), metric.WithAttributes(attrs...))
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	monitors, err := r.store.ListAllActive(ctx)
	if err != nil {
		return fmt.Errorf("list active monitors: %w", err)
	}

	slog.InfoContext(ctx, "refresh started", "monitor_count", len(monitors))

	for i := range monitors {
		if _, err := r.Refresh(ctx, monitors[i]); err != nil {
			if _, ok := errors.AsType[*Transient](err); ok {
				slog.WarnContext(ctx, "transient error refreshing monitor", "monitor_id", monitors[i].ID, "error", err)
				continue
			}
			return fmt.Errorf("refresh monitor %s: %w", monitors[i].ID, err)
		}
	}

	users := make(map[uuid.UUID]struct{}, len(monitors))
	for i := range monitors {
		users[monitors[i].UserID] = struct{}{}
	}
	r.metrics.usersRefreshed.Record(ctx, int64(len(users)))
	r.metrics.monitorsRefreshed.Record(ctx, int64(len(monitors)))
	slog.InfoContext(ctx, "refresh completed", "monitor_count", len(monitors))
	return nil
}

func persist(
	ctx context.Context,
	s AtomicStores,
	monitor Monitor,
	forecasts []Forecast,
	change RiskWindowChange,
) (*notification.Queued, error) {
	if _, err := s.ForecastStore.Save(ctx, monitor.ID, forecasts); err != nil {
		return nil, fmt.Errorf("save forecasts: %w", err)
	}

	var queued *notification.Queued
	if change.NeedsNotification() {
		msg := fogAlertMessage(monitor)
		notif := notification.New(
			monitor.UserID,
			msg,
			monitor.Location.Name,
			change.RiskWindow.Start,
			change.RiskWindow.End,
		)
		n, err := s.Outbox.Create(ctx, notif)
		if err != nil {
			return nil, fmt.Errorf("create notification: %w", err)
		}
		queued = &notification.Queued{ID: n.ID}
		slog.InfoContext(ctx, "notification queued",
			"monitor_id", monitor.ID,
			"user_id", monitor.UserID,
			"message", msg,
		)
	}

	if change.NeedsSave() {
		if _, err := s.MonitorStore.Update(ctx, monitor); err != nil {
			return nil, fmt.Errorf("update alert: %w", err)
		}
		slog.DebugContext(ctx, "monitor updated", "monitor_id", monitor.ID)
	}

	return queued, nil
}

func fogAlertMessage(m Monitor) string {
	return fmt.Sprintf("Fog is forecast for %s", m.Location.Name)
}
