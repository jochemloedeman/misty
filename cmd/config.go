package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/jochemloedeman/misty/monitor"
)

type config struct {
	DatabaseURL       string
	Port              string
	SigningSecrets    [][]byte
	LogLevel          slog.Level
	ReconcileInterval time.Duration
	ForecastHorizon   monitor.ForecastHorizon
}

func loadConfig() (config, error) {
	cfg := config{
		Port:              "8080",
		LogLevel:          slog.LevelInfo,
		ReconcileInterval: 15 * time.Minute,
		ForecastHorizon: monitor.ForecastHorizon{
			Interval: time.Hour,
			Steps:    15,
		},
	}

	cfg.DatabaseURL = os.Getenv("DATABASE_URL")
	if cfg.DatabaseURL == "" {
		return config{}, fmt.Errorf("DATABASE_URL is required")
	}

	if v := os.Getenv("PORT"); v != "" {
		cfg.Port = v
	}

	signingSecret := os.Getenv("SIGNING_SECRET")
	if signingSecret == "" {
		return config{}, fmt.Errorf("SIGNING_SECRET is required")
	}
	cfg.SigningSecrets = append(cfg.SigningSecrets, []byte(signingSecret))
	if prev := os.Getenv("SIGNING_SECRET_PREVIOUS"); prev != "" {
		cfg.SigningSecrets = append(cfg.SigningSecrets, []byte(prev))
	}

	if v := os.Getenv("LOG_LEVEL"); v != "" {
		var level slog.Level
		if err := level.UnmarshalText([]byte(v)); err != nil {
			return config{}, fmt.Errorf("invalid LOG_LEVEL %q: %w", v, err)
		}
		cfg.LogLevel = level
	}

	if v := os.Getenv("RECONCILE_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return config{}, fmt.Errorf(
				"invalid RECONCILE_INTERVAL %q: %w",
				v,
				err,
			)
		}
		cfg.ReconcileInterval = d
	}

	if v := os.Getenv("FORECAST_GRANULARITY"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return config{}, fmt.Errorf(
				"invalid FORECAST_GRANULARITY %q: %w",
				v,
				err,
			)
		}
		cfg.ForecastHorizon.Interval = d
	}

	if v := os.Getenv("FORECAST_STEPS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return config{}, fmt.Errorf("invalid FORECAST_STEPS %q: %w", v, err)
		}
		cfg.ForecastHorizon.Steps = n
	}

	return cfg, nil
}
