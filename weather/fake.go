package weather

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/jochemloedeman/misty/monitor"
)

type Clock interface {
	Now() time.Time
}

type FakeForecaster struct {
	clock                 Clock
	chanceOfFog           float64
	chanceOfForecastIfFog float64
}

func NewFakeForecaster(clock Clock, chanceOfFog, chanceOfForecastIfFog float64) FakeForecaster {
	return FakeForecaster{
		clock:                 clock,
		chanceOfFog:           chanceOfFog,
		chanceOfForecastIfFog: chanceOfForecastIfFog,
	}
}

func noFogForecast(ts time.Time) monitor.Forecast {
	return monitor.Forecast{
		Time: ts,
		WeatherVariables: monitor.WeatherVariables{
			Temperature:      15,
			DewPoint:         5,
			RelativeHumidity: 50,
			WindSpeed:        5,
			Visibility:       10000,
		},
	}
}

func fogForecast(ts time.Time) monitor.Forecast {
	return monitor.Forecast{
		Time: ts,
		WeatherVariables: monitor.WeatherVariables{
			Temperature:      10,
			DewPoint:         9,
			RelativeHumidity: 98,
			WindSpeed:        2,
			Visibility:       500,
		},
	}
}

func (f FakeForecaster) Forecast(ctx context.Context, location monitor.Location, horizon monitor.TimeHorizon) ([]monitor.Forecast, error) {
	forecasts := make([]monitor.Forecast, horizon.Steps)
	fog := rand.Float64() < f.chanceOfFog
	slog.Debug("forecast generated", "fog", fog)
	now := f.clock.Now()
	if !fog {
		for i := 0; i < horizon.Steps; i++ {
			timeOffset := time.Duration(i) * horizon.Granularity
			forecasts[i] = noFogForecast(now.Add(timeOffset))
		}
		return forecasts, nil
	}
	for i := 0; i < horizon.Steps; i++ {
		foggyForecast := rand.Float64() < f.chanceOfForecastIfFog
		slog.Debug("forecast step", "foggy", foggyForecast)
		timeOffset := time.Duration(i) * horizon.Granularity
		if foggyForecast {
			forecasts[i] = fogForecast(now.Add(timeOffset))
		} else {
			forecasts[i] = noFogForecast(now.Add(timeOffset))
		}
	}
	return forecasts, nil
}
