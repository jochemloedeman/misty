package weather

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/jochemloedeman/misty/monitor"
)

func NewStatefulNow(nrOfMonitors int, increment time.Duration) func() time.Time {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	called := 0
	return func() time.Time {
		if called%nrOfMonitors == 0 && called != 0 {
			now = now.Add(increment)
		}
		called++
		return now
	}
}

type FakeForecaster struct {
	now                   func() time.Time
	chanceOfFog           float64
	chanceOfForecastIfFog float64
}

func NewFakeForecaster(nrOfMonitors int, increment time.Duration, chanceOfFog, chanceOfForecastIfFog float64) FakeForecaster {
	return FakeForecaster{
		now:                   NewStatefulNow(nrOfMonitors, increment),
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

func (f FakeForecaster) Forecast(ctx context.Context, location monitor.Location, horizon monitor.ForecastHorizon) ([]monitor.Forecast, error) {
	forecasts := make([]monitor.Forecast, horizon.Steps)
	fog := rand.Float64() < f.chanceOfFog
	fmt.Printf("fog: %t\n", fog)
	now := f.now()
	if !fog {
		for i := 0; i < horizon.Steps; i++ {
			timeOffset := time.Duration(i) * horizon.Granularity
			forecasts[i] = noFogForecast(now.Add(timeOffset))
		}
		return forecasts, nil
	}
	foggyForecast := rand.Float64() < f.chanceOfForecastIfFog
	fmt.Printf("foggy forecast: %t\n", foggyForecast)
	for i := 0; i < horizon.Steps; i++ {
		timeOffset := time.Duration(i) * horizon.Granularity
		if foggyForecast {
			forecasts[i] = fogForecast(now.Add(timeOffset))
		} else {
			forecasts[i] = noFogForecast(now.Add(timeOffset))
		}
	}
	return forecasts, nil
}
