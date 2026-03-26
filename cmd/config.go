package main

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jochemloedeman/misty/monitor"
)

const (
	defaultReconcileMinutes = 15
	defaultForecastSteps    = 15
)

type apnsConfig struct {
	KeyPath     string
	KeyID       string
	TeamID      string
	Topic       string
	Development bool
}

type config struct {
	DatabaseURL       string
	Port              string
	SigningSecrets    [][]byte
	DevUserID         *uuid.UUID
	LogLevel          slog.Level
	ReconcileInterval time.Duration
	ForecastHorizon   monitor.ForecastHorizon
	APNS              *apnsConfig
}

func loadConfig() (config, error) { //nolint:cyclop
	cfg := config{
		Port:              "8080",
		LogLevel:          slog.LevelInfo,
		ReconcileInterval: defaultReconcileMinutes * time.Minute,
		ForecastHorizon: monitor.ForecastHorizon{
			Interval: time.Hour,
			Steps:    defaultForecastSteps,
		},
	}

	if v := os.Getenv("PORT"); v != "" {
		cfg.Port = v
	}

	postgresPasswordFile := os.Getenv("POSTGRES_PASSWORD_FILE")
	if postgresPasswordFile == "" {
		return config{}, fmt.Errorf("POSTGRES_PASSWORD_FILE is required")
	}
	postgresPassword, err := os.ReadFile(postgresPasswordFile)
	if err != nil {
		return config{}, fmt.Errorf("failed to read postgres password from file: %w", err)
	}
	postgresPassword = bytes.TrimSpace(postgresPassword)

	postgresUser := os.Getenv("POSTGRES_USER")
	if postgresUser == "" {
		return config{}, fmt.Errorf("POSTGRES_USER is required")
	}

	cfg.DatabaseURL = fmt.Sprintf(
		"postgresql://%s:%s@db:5432/postgres",
		postgresUser,
		postgresPassword,
	)

	signingSecretFile := os.Getenv("SIGNING_SECRET_FILE")
	if signingSecretFile == "" {
		return config{}, fmt.Errorf("SIGNING_SECRET_FILE is required")
	}
	signingSecret, err := os.ReadFile(signingSecretFile)
	if err != nil {
		return config{}, fmt.Errorf("failed to read signing secret from file: %w", err)
	}
	cfg.SigningSecrets = append(cfg.SigningSecrets, bytes.TrimSpace(signingSecret))

	if prev := os.Getenv("SIGNING_SECRET_PREVIOUS_FILE"); prev != "" {
		prevSecret, err := os.ReadFile(prev)
		if err != nil {
			return config{}, fmt.Errorf("failed to read previous signing secret from file: %w", err)
		}
		prevSecret = bytes.TrimSpace(prevSecret)
		if len(prevSecret) != 0 {
			cfg.SigningSecrets = append(cfg.SigningSecrets, prevSecret)
		}
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

	if v := os.Getenv("APNS_KEY_FILE"); v != "" {
		keyID := os.Getenv("APNS_KEY_ID")
		if keyID == "" {
			return config{}, fmt.Errorf("APNS_KEY_ID is required when APNS_KEY_FILE is set")
		}
		teamID := os.Getenv("APNS_TEAM_ID")
		if teamID == "" {
			return config{}, fmt.Errorf("APNS_TEAM_ID is required when APNS_KEY_FILE is set")
		}
		topic := os.Getenv("APNS_TOPIC")
		if topic == "" {
			return config{}, fmt.Errorf("APNS_TOPIC is required when APNS_KEY_FILE is set")
		}
		cfg.APNS = &apnsConfig{
			KeyPath:     v,
			KeyID:       keyID,
			TeamID:      teamID,
			Topic:       topic,
			Development: os.Getenv("APNS_DEVELOPMENT") == "true",
		}
	}

	if v := os.Getenv("DEV_USER_ID"); v != "" {
		uid, err := uuid.Parse(v)
		if err != nil {
			return config{}, fmt.Errorf("invalid DEV_USER_ID %q: %w", v, err)
		}
		cfg.DevUserID = &uid
	}

	return cfg, nil
}
