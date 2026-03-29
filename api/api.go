package api

import (
	"cmp"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jochemloedeman/misty/auth"
	"github.com/jochemloedeman/misty/monitor"
	"github.com/jochemloedeman/misty/user"
)

type contextKey string

const userIDKey contextKey = "userID"

type MonitorStore interface {
	List(ctx context.Context) ([]monitor.Monitor, error)
	Get(ctx context.Context, monitorID uuid.UUID) (monitor.Monitor, error)
	Create(ctx context.Context, m monitor.Monitor) (monitor.Monitor, error)
	Update(ctx context.Context, m monitor.Monitor) (monitor.Monitor, error)
	Delete(ctx context.Context, monitorID uuid.UUID) error
}

type UserStore interface {
	Create(ctx context.Context, u user.User) (user.User, error)
	GetByRefreshToken(
		ctx context.Context,
		refreshToken string,
	) (user.User, error)
	UpdatePushToken(ctx context.Context, userID uuid.UUID, pushToken string) (user.User, error)
}

type ForecastStore interface {
	ListForMonitorInRange(
		ctx context.Context,
		monitorID uuid.UUID,
		from, until time.Time,
	) ([]monitor.Forecast, error)
}

type TokenVerifier interface {
	Verify(token string) (*auth.Claims, error)
}

type TokenIssuer interface {
	Issue(userID uuid.UUID) (string, error)
}

type LocationResponse struct {
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
}

type AlertResponse struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type MonitorResponse struct {
	ID       uuid.UUID        `json:"id"`
	IsActive bool             `json:"is_active"`
	Location LocationResponse `json:"location"`
	Alert    *AlertResponse   `json:"alert,omitempty"`
}

func toMonitorResponse(m monitor.Monitor) MonitorResponse {
	res := MonitorResponse{
		ID:       m.ID,
		IsActive: m.IsActive,
		Location: LocationResponse{
			Name: m.Location.Name,
			Lat:  m.Location.Lat,
			Lon:  m.Location.Lon,
		},
	}
	if m.ActiveAlert != nil {
		res.Alert = &AlertResponse{
			Start: m.ActiveAlert.Start.UTC().Format(time.RFC3339),
			End:   m.ActiveAlert.End.UTC().Format(time.RFC3339),
		}
	}
	return res
}

type ForecastResponse struct {
	ForecastAt       string  `json:"forecast_at"`
	Temperature      float64 `json:"temperature"`
	DewPoint         float64 `json:"dew_point"`
	RelativeHumidity float64 `json:"relative_humidity"`
	WindSpeed        float64 `json:"wind_speed"`
	Visibility       float64 `json:"visibility"`
	WeatherCode      int     `json:"weather_code"`
	IsFogLikely      bool    `json:"is_fog_likely"`
}

type API struct {
	newMonitorStore func(userID uuid.UUID) MonitorStore
	forecastStore   ForecastStore
	userStore       UserStore
	issuer          TokenIssuer
	onCreated       func(monitor.Monitor)
	now             func() time.Time
}

func New(
	userStore UserStore,
	newMonitorStore func(userID uuid.UUID) MonitorStore,
	forecastStore ForecastStore,
	issuer TokenIssuer,
	onCreated func(monitor.Monitor),
	now func() time.Time,
) *API {
	return &API{
		userStore:       userStore,
		newMonitorStore: newMonitorStore,
		forecastStore:   forecastStore,
		issuer:          issuer,
		onCreated:       onCreated,
		now:             now,
	}
}

func userID(ctx context.Context) uuid.UUID {
	// panics is intentional here. If the userID is not set,
	// it means RequireUser middleware was not used, which is a programming error.
	return ctx.Value(userIDKey).(uuid.UUID)
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

type errorOption func(http.ResponseWriter, *ErrorResponse)

func withMessage(msg string) errorOption {
	return func(_ http.ResponseWriter, r *ErrorResponse) {
		r.Message = msg
	}
}

func withHeader(key, value string) errorOption {
	return func(w http.ResponseWriter, _ *ErrorResponse) {
		w.Header().Add(key, value)
	}
}

func writeError(w http.ResponseWriter, status int, opts ...errorOption) {
	resp := ErrorResponse{Error: http.StatusText(status)}
	for _, opt := range opts {
		opt(w, &resp)
	}
	writeJSON(w, status, resp)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("Failed to write JSON response", "error", err)
	}
}

func (s *API) HealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (s *API) ListMonitors(w http.ResponseWriter, r *http.Request) {
	store := s.newMonitorStore(userID(r.Context()))

	monitors, err := store.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError)
		return
	}
	res := make([]MonitorResponse, len(monitors))
	for i, m := range monitors {
		res[i] = toMonitorResponse(m)
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *API) GetMonitor(w http.ResponseWriter, r *http.Request) {
	store := s.newMonitorStore(userID(r.Context()))

	monitorID := r.PathValue("id")
	mid, err := uuid.Parse(monitorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, withMessage("invalid monitor id"))
		return
	}

	m, err := store.Get(r.Context(), mid)
	if errors.Is(err, monitor.ErrNotFound) {
		writeError(w, http.StatusNotFound)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError)
		return
	}

	res := toMonitorResponse(m)
	writeJSON(w, http.StatusOK, res)
}

func (s *API) ListForecasts(w http.ResponseWriter, r *http.Request) {
	store := s.newMonitorStore(userID(r.Context()))

	mid, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, withMessage("invalid monitor id"))
		return
	}

	_, err = store.Get(r.Context(), mid)
	if errors.Is(err, monitor.ErrNotFound) {
		writeError(w, http.StatusNotFound)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError)
		return
	}

	horizonStr := cmp.Or(r.URL.Query().Get("horizon"), "12h")
	horizon, err := time.ParseDuration(horizonStr)
	if err != nil || horizon <= 0 || horizon > 48*time.Hour {
		writeError(w, http.StatusBadRequest, withMessage("invalid horizon, must be a positive duration up to 48h"))
		return
	}

	now := s.now().Truncate(time.Hour)
	forecasts, err := s.forecastStore.ListForMonitorInRange(r.Context(), mid, now, now.Add(horizon))
	if err != nil {
		writeError(w, http.StatusInternalServerError)
		return
	}

	res := make([]ForecastResponse, len(forecasts))
	for i, f := range forecasts {
		res[i] = ForecastResponse{
			ForecastAt:       f.Time.UTC().Format(time.RFC3339),
			Temperature:      f.Temperature,
			DewPoint:         f.DewPoint,
			RelativeHumidity: f.RelativeHumidity,
			WindSpeed:        f.WindSpeed,
			Visibility:       f.Visibility,
			WeatherCode:      f.WeatherCode,
			IsFogLikely:      f.IsFogLikely(),
		}
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *API) CreateMonitor(w http.ResponseWriter, r *http.Request) {
	uid := userID(r.Context())
	store := s.newMonitorStore(uid)

	type params struct {
		LocationName string  `json:"location_name"`
		Lat          float64 `json:"latitude"`
		Lon          float64 `json:"longitude"`
	}
	var p params
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		slog.Debug("failed to decode request body", "error", err)
		writeError(
			w,
			http.StatusBadRequest,
			withMessage("invalid request body"),
		)
		return
	}

	m := monitor.NewMonitor(
		uid,
		monitor.Location{
			Name: p.LocationName,
			Lat:  p.Lat,
			Lon:  p.Lon,
		},
	)
	created, err := store.Create(r.Context(), m)
	if err != nil {
		writeError(w, http.StatusInternalServerError)
		return
	}

	slog.Info(
		"monitor created",
		"monitor_id",
		created.ID,
		"user_id",
		uid,
		"location",
		created.Location.Name,
	)

	res := toMonitorResponse(created)
	writeJSON(w, http.StatusCreated, res)

	s.onCreated(created)
}

func (s *API) SetMonitorStatus(activate bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store := s.newMonitorStore(userID(r.Context()))

		mid, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			writeError(
				w,
				http.StatusBadRequest,
				withMessage("invalid monitor id"),
			)
			return
		}

		m, err := store.Get(r.Context(), mid)
		if errors.Is(err, monitor.ErrNotFound) {
			writeError(w, http.StatusNotFound)
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError)
			return
		}

		if activate {
			m = m.Activate()
		} else {
			m = m.Deactivate()
		}

		updated, err := store.Update(r.Context(), m)
		if errors.Is(err, monitor.ErrNotFound) {
			writeError(w, http.StatusNotFound)
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError)
			return
		}

		slog.Info(
			"monitor status changed",
			"monitor_id",
			updated.ID,
			"is_active",
			updated.IsActive,
		)

		writeJSON(w, http.StatusOK, toMonitorResponse(updated))
	}
}

func (s *API) DeleteMonitor(w http.ResponseWriter, r *http.Request) {
	store := s.newMonitorStore(userID(r.Context()))

	mid, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, withMessage("invalid monitor id"))
		return
	}

	uid := userID(r.Context())

	err = store.Delete(r.Context(), mid)
	if errors.Is(err, monitor.ErrNotFound) {
		writeError(w, http.StatusNotFound)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError)
		return
	}

	slog.Info("monitor deleted", "monitor_id", mid, "user_id", uid)
	w.WriteHeader(http.StatusNoContent)
}

func (s *API) Register(w http.ResponseWriter, r *http.Request) {
	u, plainRefreshToken, err := user.New()
	if err != nil {
		writeError(w, http.StatusInternalServerError)
		return
	}
	created, err := s.userStore.Create(r.Context(), u)
	if err != nil {
		writeError(w, http.StatusInternalServerError)
		return
	}

	accessToken, err := s.issuer.Issue(created.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError)
		return
	}

	slog.Info("user registered", "user_id", created.ID)

	type response struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	writeJSON(w, http.StatusCreated, response{
		AccessToken:  accessToken,
		RefreshToken: plainRefreshToken,
	})
}

func (s *API) TokenRefresh(w http.ResponseWriter, r *http.Request) {
	type params struct {
		RefreshToken string `json:"refresh_token"`
	}
	var p params
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		slog.Debug("failed to decode request body", "error", err)
		writeError(
			w,
			http.StatusBadRequest,
			withMessage("invalid request body"),
		)
		return
	}

	u, err := s.userStore.GetByRefreshToken(r.Context(), p.RefreshToken)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			writeError(
				w,
				http.StatusUnauthorized,
				withMessage("invalid refresh token"),
			)
			return
		}
		writeError(w, http.StatusInternalServerError)
		return
	}
	accessToken, err := s.issuer.Issue(u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError)
		return
	}

	type response struct {
		AccessToken string `json:"access_token"`
	}
	writeJSON(w, http.StatusOK, response{
		AccessToken: accessToken,
	})
}

func (s *API) UpdatePushToken(w http.ResponseWriter, r *http.Request) {
	type Params struct {
		PushToken string `json:"push_token"`
	}
	var p Params
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		slog.Debug("decoding push token", "error", err)
		writeError(
			w,
			http.StatusBadRequest,
			withMessage("invalid request body"),
		)
		return
	}
	
	_, err := hex.DecodeString(p.PushToken)
	if err != nil {
		slog.Debug("decoding push token hex", "error", err)
		writeError(
			w,
			http.StatusBadRequest,
			withMessage("invalid push token format"),
		)
		return
	}

	uid := userID(r.Context())
	updated, err := s.userStore.UpdatePushToken(r.Context(), uid, p.PushToken)
	if errors.Is(err, user.ErrNotFound) {
		writeError(w, http.StatusNotFound)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError)
		return
	}
	
	slog.Info("push token updated", "user_id", updated.ID)
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})	
}