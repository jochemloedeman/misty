package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jochemloedeman/misty/monitor"
	"github.com/jochemloedeman/misty/users"
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
	Create(ctx context.Context, u users.User) (users.User, error)
	GetByRefreshToken(
		ctx context.Context,
		refreshToken string,
	) (users.User, error)
}

type TokenVerifier interface {
	Verify(token string) (*users.Claims, error)
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

type API struct {
	newMonitorStore func(userID uuid.UUID) MonitorStore
	userStore       UserStore
	issuer          TokenIssuer
}

func New(
	userStore UserStore,
	newMonitorStore func(userID uuid.UUID) MonitorStore,
	issuer TokenIssuer,
) *API {
	return &API{
		userStore:       userStore,
		newMonitorStore: newMonitorStore,
		issuer:          issuer,
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
	type params struct {
		PushToken string `json:"push_token"`
	}
	var p params
	if err := json.NewDecoder(r.Body).
		Decode(&p); err != nil &&
		!errors.Is(err, io.EOF) {
		slog.Debug("failed to decode request body", "error", err)
		writeError(
			w,
			http.StatusBadRequest,
			withMessage("invalid request body"),
		)
		return
	}

	u, plainRefreshToken, err := users.NewUser(p.PushToken)
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

	user, err := s.userStore.GetByRefreshToken(r.Context(), p.RefreshToken)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
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
	accessToken, err := s.issuer.Issue(user.ID)
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
