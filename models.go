package main

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ForecastHorizon int

const (
	horizon ForecastHorizon = 12
)

type WeatherVariables struct {
	Temperature      float64 `unit:"°C"`
	DewPoint         float64 `unit:"°C"`
	RelativeHumidity float64 `unit:"%"`
	WindSpeed        float64 `unit:"m/s"`
	Visibility       int     `unit:"m"`
}

func (w WeatherVariables) FogIsLikely() bool {
	dewPointClose := (w.Temperature - w.DewPoint) < 2.5
	poorVisibility := w.Visibility < 1000
	highHumidity := w.RelativeHumidity > 95
	calmWinds := w.WindSpeed < 10
	return dewPointClose && poorVisibility && highHumidity && calmWinds
}

type TimeOfDay struct {
	Hour   int
	Minute int
}

func NewTimeOfDay(hour int, minute int) (TimeOfDay, error) {
	if hour < 0 || hour > 23 {
		return TimeOfDay{}, fmt.Errorf("invalid hour: %d", hour)
	}
	if minute < 0 || minute > 59 {
		return TimeOfDay{}, fmt.Errorf("invalid minute: %d", minute)
	}
	return TimeOfDay{Hour: hour, Minute: minute}, nil
}

type DailyWindow struct {
	Start TimeOfDay
	End   TimeOfDay
}

type Location struct {
	Name      string
	Latitude  float64
	Longitude float64
}

type Monitor struct {
	ID       uuid.UUID
	Location Location
	IsActive bool
	DailyWindow
}

type Forecast struct {
	ForecastedAt time.Time
	MonitorID    uuid.UUID
	WeatherVariables
}

type Alert struct {
	ID        uuid.UUID
	MonitorID uuid.UUID
	Start     time.Time
	End       time.Time
}
