package monitor

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jochemloedeman/misty/monitor/sqlc"
)

func toDomainForecast(row sqlc.Forecast) Forecast {
	f := Forecast{
		Time: row.ForecastAt.Time,
		WeatherVariables: WeatherVariables{
			Temperature:      row.Temperature,
			DewPoint:         row.DewPoint,
			RelativeHumidity: row.RelativeHumidity,
			WindSpeed:        row.WindSpeed,
			Visibility:       int(row.Visibility),
		},
	}
	return f

}

type PostgresForecastStore struct {
	queries *sqlc.Queries
}

func NewPostgresForecastStore(queries *sqlc.Queries) *PostgresForecastStore {
	return &PostgresForecastStore{queries: queries}
}

func (s *PostgresForecastStore) ListForMonitor(ctx context.Context, monitorID uuid.UUID) ([]Forecast, error) {
	rows, err := s.queries.ListForecastsByMonitorID(ctx, dbUUID(monitorID))
	if err != nil {
		return nil, fmt.Errorf("failed to list forecasts: %w", err)
	}
	forecasts := make([]Forecast, len(rows))
	for i, row := range rows {
		forecasts[i] = toDomainForecast(row)
	}
	return forecasts, nil

}

func (s *PostgresForecastStore) Save(ctx context.Context, monitorID uuid.UUID, forecasts []Forecast) ([]Forecast, error) {
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
	savedForecasts := make([]Forecast, len(forecasts))

	for i := range params {
		row, err := s.queries.UpsertForecast(ctx, params[i])
		if err != nil {
			return nil, fmt.Errorf("failed to upsert forecast: %w", err)
		}
		savedForecasts[i] = toDomainForecast(row)
	}

	return savedForecasts, nil
}
