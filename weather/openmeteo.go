package weather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jochemloedeman/misty/monitor"
)

const (
	openMeteoBaseURL    = "https://api.open-meteo.com/v1/forecast"
	quarterHourInterval = 15 * time.Minute
	maxErrorBodyBytes   = 512
)

var variableValues = strings.Join(
	[]string{
		"temperature_2m",
		"dew_point_2m",
		"relative_humidity_2m",
		"wind_speed_10m",
		"visibility",
		"weather_code",
	},
	",",
)

type intervalConfig struct {
	horizonKey  string
	variableKey string
	pickResp    func(response) variablesResponse
}

func buildURL(
	location monitor.Location,
	horizon monitor.ForecastHorizon,
	intconf intervalConfig,
) string {
	q := make(url.Values)
	q.Set("latitude", strconv.FormatFloat(location.Lat, 'f', 2, 64))
	q.Set("longitude", strconv.FormatFloat(location.Lon, 'f', 2, 64))
	q.Set(intconf.horizonKey, strconv.Itoa(horizon.Steps))
	q.Set(intconf.variableKey, variableValues)
	q.Set("timezone", "UTC")

	return openMeteoBaseURL + "?" + q.Encode()
}

func allEqual(values ...int) bool {
	for _, v := range values[1:] {
		if v != values[0] {
			return false
		}
	}

	return true
}

type variablesResponse struct {
	Time             []string  `json:"time"`
	Temperature      []float64 `json:"temperature_2m"`
	DewPoint         []float64 `json:"dew_point_2m"`
	RelativeHumidity []float64 `json:"relative_humidity_2m"`
	WindSpeed        []float64 `json:"wind_speed_10m"`
	Visibility       []float64 `json:"visibility"`
	WeatherCode      []float64 `json:"weather_code"`
}

type response struct {
	Hourly      variablesResponse `json:"hourly"`
	Minutely15M variablesResponse `json:"minutely_15m"`
}

type Forecaster struct {
	client *http.Client
}

func NewForecaster(client *http.Client) *Forecaster {
	return &Forecaster{client: client}
}

func (f *Forecaster) Forecast(
	ctx context.Context,
	location monitor.Location,
	horizon monitor.ForecastHorizon,
) ([]monitor.Forecast, error) {
	var intconf intervalConfig
	switch horizon.Interval {
	case time.Hour:
		intconf = intervalConfig{
			horizonKey:  "forecast_hours",
			variableKey: "hourly",
			pickResp: func(resp response) variablesResponse {
				return resp.Hourly
			},
		}
	case quarterHourInterval:
		intconf = intervalConfig{
			horizonKey:  "forecast_minutely_15m",
			variableKey: "minutely_15m",
			pickResp: func(resp response) variablesResponse {
				return resp.Minutely15M
			},
		}
	default:
		return nil, fmt.Errorf("unsupported interval: %s", horizon.Interval)
	}

	reqURL := buildURL(location, horizon, intconf)
	slog.DebugContext(
		ctx,
		"fetching forecast",
		"location",
		location.Name,
		"interval",
		horizon.Interval,
		"steps",
		horizon.Steps,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return nil, fmt.Errorf(
			"unexpected status %d: %s",
			resp.StatusCode,
			body,
		)
	}

	var apiResp response
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decoding response body: %w", err)
	}
	vars := intconf.pickResp(apiResp)
	if !allEqual(
		len(vars.Time),
		len(vars.Temperature),
		len(vars.DewPoint),
		len(vars.RelativeHumidity),
		len(vars.WindSpeed),
		len(vars.Visibility),
		len(vars.WeatherCode),
	) {
		return nil, errors.New("inconsistent response lengths")
	}

	forecasts := make([]monitor.Forecast, len(vars.Time))
	for i := range vars.Time {
		casted, err := time.Parse("2006-01-02T15:04", vars.Time[i])
		if err != nil {
			return nil, fmt.Errorf("parsing time: %w", err)
		}
		forecasts[i] = monitor.Forecast{
			Time: casted,
			WeatherVariables: monitor.WeatherVariables{
				Temperature:      vars.Temperature[i],
				DewPoint:         vars.DewPoint[i],
				RelativeHumidity: vars.RelativeHumidity[i],
				WindSpeed:        vars.WindSpeed[i],
				Visibility:       vars.Visibility[i],
				WeatherCode:      int(vars.WeatherCode[i]),
			},
		}
	}
	slog.InfoContext(
		ctx,
		"forecast retrieved",
		"location",
		location.Name,
		"forecast_count",
		len(forecasts),
	)

	return forecasts, nil
}
