package weather

import (
	"context"
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

func NewFakeForecaster(
	clock Clock,
	chanceOfFog, chanceOfForecastIfFog float64,
) FakeForecaster {
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

func (f FakeForecaster) Forecast(
	ctx context.Context,
	location monitor.Location,
	horizon monitor.ForecastHorizon,
) ([]monitor.Forecast, error) {
	forecasts := make([]monitor.Forecast, horizon.Steps)
	fog := rand.Float64() < f.chanceOfFog
	now := f.clock.Now()
	dayStart := time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		0,
		0,
		0,
		0,
		now.Location(),
	)

	if !fog {
		for i := 0; i < horizon.Steps; i++ {
			timeOffset := time.Duration(i) * horizon.Interval
			forecasts[i] = noFogForecast(dayStart.Add(timeOffset))
		}
		return forecasts, nil
	}
	for i := 0; i < horizon.Steps; i++ {
		foggyForecast := rand.Float64() < f.chanceOfForecastIfFog
		timeOffset := time.Duration(i) * horizon.Interval
		if foggyForecast {
			forecasts[i] = fogForecast(dayStart.Add(timeOffset))
		} else {
			forecasts[i] = noFogForecast(dayStart.Add(timeOffset))
		}
	}
	return forecasts, nil
}
