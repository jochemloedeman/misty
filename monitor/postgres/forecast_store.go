package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jochemloedeman/misty/monitor"
	"github.com/jochemloedeman/misty/db/sqlc"
)

func toDomainForecast(row sqlc.Forecast) monitor.Forecast {
	return monitor.Forecast{
		Time: row.ForecastAt.Time,
		WeatherVariables: monitor.WeatherVariables{
			Temperature:      row.Temperature,
			DewPoint:         row.DewPoint,
			RelativeHumidity: row.RelativeHumidity,
			WindSpeed:        row.WindSpeed,
			Visibility:       int(row.Visibility),
		},
	}
}

type ForecastStore struct {
	queries *sqlc.Queries
}

func NewForecastStore(queries *sqlc.Queries) *ForecastStore {
	return &ForecastStore{queries: queries}
}

func (s *ForecastStore) ListForMonitor(ctx context.Context, monitorID uuid.UUID) ([]monitor.Forecast, error) {
	rows, err := s.queries.ListForecastsByMonitorID(ctx, dbUUID(monitorID))
	if err != nil {
		return nil, fmt.Errorf("failed to list forecasts: %w", err)
	}
	forecasts := make([]monitor.Forecast, len(rows))
	for i, row := range rows {
		forecasts[i] = toDomainForecast(row)
	}
	return forecasts, nil
}

func (s *ForecastStore) Save(ctx context.Context, monitorID uuid.UUID, forecasts []monitor.Forecast) ([]monitor.Forecast, error) {
	params := make([]sqlc.UpsertForecastParams, len(forecasts))
	for i, forecast := range forecasts {
		params[i] = sqlc.UpsertForecastParams{
			ForecastAt:       dbTime(forecast.Time),
			Temperature:      forecast.WeatherVariables.Temperature,
			DewPoint:         forecast.WeatherVariables.DewPoint,
			RelativeHumidity: forecast.WeatherVariables.RelativeHumidity,
			WindSpeed:        forecast.WeatherVariables.WindSpeed,
			Visibility:       int32(forecast.WeatherVariables.Visibility),
			MonitorID:        dbUUID(monitorID),
		}
	}
	savedForecasts := make([]monitor.Forecast, len(forecasts))

	for i := range params {
		row, err := s.queries.UpsertForecast(ctx, params[i])
		if err != nil {
			return nil, fmt.Errorf("failed to upsert forecast: %w", err)
		}
		savedForecasts[i] = toDomainForecast(row)
	}

	return savedForecasts, nil
}
