package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jochemloedeman/misty/api"
	"github.com/jochemloedeman/misty/auth"
	"github.com/jochemloedeman/misty/clock"
	"github.com/jochemloedeman/misty/db"
	"github.com/jochemloedeman/misty/db/sqlc"
	"github.com/jochemloedeman/misty/monitor"
	"github.com/jochemloedeman/misty/notification"
	"github.com/jochemloedeman/misty/notification/apple"
	"github.com/jochemloedeman/misty/user"
	"github.com/jochemloedeman/misty/weather"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/token"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

const (
	shutdownTimeout = 10 * time.Second
	maxBodySize     = 4 << 10
)

var (
	tracer             = otel.Tracer("github.com/jochemloedeman/misty/cmd")
	meter              = otel.Meter("github.com/jochemloedeman/misty/cmd")
	durationBoundaries = []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5}
)

func requestFilter(r *http.Request) bool {
	if slices.Contains([]string{"/health", "/metrics"}, r.URL.Path) {
		return false
	}
	return true
}

type metrics struct {
	refreshDuration   metric.Float64Histogram
	monitorsRefreshed metric.Int64Gauge
	usersRefreshed    metric.Int64Gauge
}

func newMetrics() (*metrics, error) {
	var err, e error
	refreshDuration, e := meter.Float64Histogram(
		"refresh.duration",
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBoundaries...),
	)
	if e != nil {
		err = errors.Join(err, e)
	}
	monitorsRefreshed, e := meter.Int64Gauge(
		"monitors.refreshed",
		metric.WithUnit("{monitor}"),
	)
	if e != nil {
		err = errors.Join(err, e)
	}
	usersRefreshed, e := meter.Int64Gauge(
		"users.refreshed",
		metric.WithUnit("{user}"),
	)
	if e != nil {
		err = errors.Join(err, e)
	}
	return &metrics{
		refreshDuration:   refreshDuration,
		monitorsRefreshed: monitorsRefreshed,
		usersRefreshed:    usersRefreshed,
	}, err
}

func runServer(
	ctx context.Context,
	routes *api.API,
	verifier api.TokenVerifier,
	port string,
) error {
	mux := http.NewServeMux()

	requireUser := api.RequireUser(verifier)
	protected := func(h http.HandlerFunc) http.HandlerFunc {
		return requireUser(h).ServeHTTP
	}

	// protected routes
	mux.HandleFunc("GET /monitors", protected(routes.ListMonitors))
	mux.HandleFunc("GET /monitors/{id}", protected(routes.GetMonitor))
	mux.HandleFunc(
		"GET /monitors/{id}/forecasts",
		protected(routes.ListForecasts),
	)
	mux.HandleFunc("POST /monitors", protected(routes.CreateMonitor))
	mux.HandleFunc(
		"POST /monitors/{id}/deactivate",
		protected(routes.SetMonitorStatus(false)),
	)
	mux.HandleFunc(
		"POST /monitors/{id}/activate",
		protected(routes.SetMonitorStatus(true)),
	)
	mux.HandleFunc("DELETE /monitors/{id}", protected(routes.DeleteMonitor))
	mux.HandleFunc("PUT /device", protected(routes.UpdatePushToken))

	// bare route
	mux.HandleFunc("POST /register", routes.Register)
	mux.HandleFunc("POST /token/refresh", routes.TokenRefresh)
	mux.HandleFunc("GET /health", routes.HealthCheck)
	mux.Handle("/metrics", promhttp.Handler())

	instrumented := otelhttp.NewHandler(
		mux,
		"server",
		otelhttp.WithFilter(requestFilter),
	)
	srv := &http.Server{
		Addr: ":" + port,
		Handler: api.Compose(
			api.RequestLogger,
			api.MaxBodySize(maxBodySize),
		)(instrumented),
	}

	go func() {
		<-ctx.Done()
		slog.InfoContext(ctx, "http server shutting down")

		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			shutdownTimeout,
		)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.InfoContext(ctx, "http server listening", "addr", srv.Addr)

	err := srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func traceRefresh(
	ctx context.Context,
	refresher *monitor.Refresher,
	m monitor.Monitor,
	horizon monitor.ForecastHorizon,
) (err error) {
	ctx, span := tracer.Start(ctx, "refresh", trace.WithAttributes(
		attribute.String("monitor.id", m.ID.String()),
		attribute.String("monitor.location", m.Location.Name),
	))
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()
	return refresher.Refresh(ctx, m, horizon)
}

func traceRefreshAll(
	ctx context.Context,
	store monitor.MonitorStore,
	refresher *monitor.Refresher,
	horizon monitor.ForecastHorizon,
	metrics *metrics,
) (err error) {
	ctx, span := tracer.Start(ctx, "refresh.all")
	start := time.Now()
	defer func() {
		var attrs []attribute.KeyValue
		if err != nil {
			attrs = append(attrs, semconv.ErrorTypeKey.String("refresh_failed"))
		}
		metrics.refreshDuration.Record(
			ctx,
			time.Since(start).Seconds(),
			metric.WithAttributes(attrs...),
		)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()
	return refreshAllMonitors(ctx, store, refresher, horizon, metrics)
}

func refreshAllMonitors(
	ctx context.Context,
	store monitor.MonitorStore,
	refresher *monitor.Refresher,
	horizon monitor.ForecastHorizon,
	metrics *metrics,
) error {
	monitors, err := store.ListAllActive(ctx)
	if err != nil {
		return fmt.Errorf("list active monitors: %w", err)
	}

	slog.InfoContext(ctx, "refresh started", "monitor_count", len(monitors))

	for i := range monitors {
		if err := traceRefresh(ctx, refresher, monitors[i], horizon); err != nil {
			if _, ok := errors.AsType[*monitor.Transient](err); ok {
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
	}

	users := make(map[uuid.UUID]struct{}, len(monitors))
	for i := range monitors {
		users[monitors[i].UserID] = struct{}{}
	}
	metrics.usersRefreshed.Record(ctx, int64(len(users)))
	metrics.monitorsRefreshed.Record(ctx, int64(len(monitors)))
	slog.InfoContext(ctx, "refresh completed", "monitor_count", len(monitors))
	return nil
}

func runRefreshLoop(
	ctx context.Context,
	monitorStore monitor.MonitorStore,
	refresher *monitor.Refresher,
	dispatcher *RefreshDispatcher,
	notifier *notification.Notifier,
	interval time.Duration,
	horizon monitor.ForecastHorizon,
	clock clock.RealClock,
	metrics *metrics,
) error {
	ticker := clock.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := traceRefreshAll(ctx, monitorStore, refresher, horizon, metrics); err != nil {
				slog.ErrorContext(ctx, "refresh error", "error", err)
			}
		case request := <-dispatcher.Incoming():
			ctx := request.Context(ctx)
			if err := traceRefresh(
				ctx,
				refresher,
				request.monitor,
				horizon,
			); err != nil {
				slog.ErrorContext(
					ctx,
					"immediate refresh failed",
					"monitor_id",
					request.monitor.ID,
					"error",
					err,
				)
			}
		case <-ctx.Done():
			return nil
		}

		if err := notifier.Notify(ctx); err != nil {
			slog.ErrorContext(ctx, "notification error", "error", err)
		}
	}
}

func checkHealth(port string) {
	resp, err := http.Get("http://localhost:" + port + "/health")
	if err != nil {
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		os.Exit(1)
	}
}

func run() (err error) {
	cfg, err := loadConfig()
	if err != nil {
		slog.ErrorContext(
			context.Background(),
			"invalid configuration",
			"error",
			err,
		)
		os.Exit(1)
	}

	slog.SetDefault(slog.New(fanout{
		otelslog.NewHandler("misty"),
		slog.NewTextHandler(
			os.Stderr,
			&slog.HandlerOptions{Level: cfg.LogLevel},
		),
	}))

	slog.InfoContext(context.Background(), "starting misty",
		"port", cfg.Port,
		"log_level", cfg.LogLevel,
		"refresh_interval", cfg.RefreshInterval,
		"forecast_horizon", cfg.ForecastHorizon,
	)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	otelShutdown, err := setupOTelSDK(ctx)
	if err != nil {
		return fmt.Errorf("failed to setup open telemtry: %w", err)
	}
	defer func() {
		err = errors.Join(err, otelShutdown(ctx))
	}()

	metrics, err := newMetrics()
	if err != nil {
		return fmt.Errorf("creating metrics: %w", err)
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("parse database config: %w", err)
	}
	poolCfg.ConnConfig.Tracer = otelpgx.NewTracer()

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("creating database pool: %w", err)
	}
	defer pool.Close()

	err = db.Migrate(ctx, pool)
	if err != nil {
		return fmt.Errorf("database migration failed: %w", err)
	}

	slog.InfoContext(ctx, "database connected")

	clk := clock.NewRealClock()

	queries := sqlc.New(pool)
	userStore := user.NewStore(queries)
	monitorStore := monitor.NewMonitorStore(queries)

	refreshDispatcher := NewRefreshDispatcher()
	refresher, err := monitor.NewRefresher(
		weather.NewForecaster(
			&http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)},
		),
		monitor.NewRunAtomically(pool),
		clk,
	)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create refresher", "error", err)
		os.Exit(1)
	}

	keyRing, err := auth.NewKeyRing(cfg.SigningSecrets, clk.Now)
	if err != nil {
		slog.ErrorContext(ctx, "invalid key ring configuration", "error", err)
		os.Exit(1)
	}
	forecastStore := monitor.NewForecastStore(queries)
	routes := api.New(
		userStore,
		monitorStore,
		forecastStore,
		keyRing,
		refreshDispatcher.Request,
		clk.Now,
		cfg.MonitorLimit,
	)

	var deliverFn func(context.Context, notification.Fog) error
	if cfg.APNS != nil {
		authKey, err := token.AuthKeyFromFile(cfg.APNS.KeyPath)
		if err != nil {
			slog.ErrorContext(ctx, "invalid APNs auth key", "error", err)
			os.Exit(1)
		}
		tok := &token.Token{
			AuthKey: authKey,
			KeyID:   cfg.APNS.KeyID,
			TeamID:  cfg.APNS.TeamID,
		}
		apnsClient := apns2.NewTokenClient(tok)
		if cfg.APNS.Development {
			apnsClient = apnsClient.Development()
		} else {
			apnsClient = apnsClient.Production()
		}
		apnsClient.HTTPClient.Transport = otelhttp.NewTransport(apnsClient.HTTPClient.Transport)
		deliverFn = apple.NewDeliverer(
			apnsClient,
			apple.NewPGTokenResolver(queries),
			cfg.APNS.Topic,
		)
		slog.InfoContext(
			ctx,
			"APNs delivery enabled",
			"topic",
			cfg.APNS.Topic,
			"environment",
			map[bool]string{true: "development", false: "production"}[cfg.APNS.Development],
		)
	} else {
		slog.WarnContext(
			ctx,
			"APNs not configured — notifications will be logged only",
		)
		deliverFn = func(ctx context.Context, notif notification.Fog) error {
			slog.InfoContext(ctx, "notification delivered (no-op)",
				"notification_id", notif.ID,
				"recipient_id", notif.RecipientID,
			)
			return nil
		}
	}

	notifier, err := notification.NewNotifier(
		notification.NewOutbox(queries),
		deliverFn,
		clk.Now,
	)
	if err != nil {
		return fmt.Errorf("failed to create notifier: %w", err)
	}

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return runServer(ctx, routes, keyRing, cfg.Port)
	})
	group.Go(func() error {
		return runRefreshLoop(
			ctx,
			monitorStore,
			refresher,
			refreshDispatcher,
			notifier,
			cfg.RefreshInterval,
			cfg.ForecastHorizon,
			clk,
			metrics,
		)
	})

	if err := group.Wait(); err != nil {
		slog.ErrorContext(ctx, "application error", "error", err)
		return err
	}
	return nil
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "health" {
		checkHealth("8080")
		return
	}
	if err := run(); err != nil {
		slog.ErrorContext(
			context.Background(),
			"application error",
			"error",
			err,
		)
		os.Exit(1)
	}
}
