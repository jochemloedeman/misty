package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jochemloedeman/misty/monitor"
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
	newStore func(userID uuid.UUID) MonitorStore
}

func New(newStore func(userID uuid.UUID) MonitorStore) *API {
	return &API{
		newStore: newStore,
	}
}

func userID(ctx context.Context) uuid.UUID {
	// panics is intentional here. If the userID is not set,
	// it means RequireUser middleware was not used, which is a programming error.
	return ctx.Value(userIDKey).(uuid.UUID)
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, status int, err error) {
	if status >= 500 {
		slog.Error("Server error", "status", status, "error", err)
	}
	writeJSON(w, status, ErrorResponse{Error: http.StatusText(status)})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("Failed to write JSON response", "error", err)
	}
}

func (s *API) ListMonitors(w http.ResponseWriter, r *http.Request) {
	store := s.newStore(userID(r.Context()))

	monitors, err := store.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	res := make([]MonitorResponse, len(monitors))
	for i, m := range monitors {
		res[i] = toMonitorResponse(m)
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *API) GetMonitor(w http.ResponseWriter, r *http.Request) {
	store := s.newStore(userID(r.Context()))

	monitorID := r.PathValue("id")
	mid, err := uuid.Parse(monitorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	m, err := store.Get(r.Context(), mid)
	if errors.Is(err, monitor.ErrNotFound) {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	res := toMonitorResponse(m)
	writeJSON(w, http.StatusOK, res)
}

func (s *API) CreateMonitor(w http.ResponseWriter, r *http.Request) {
	uid := userID(r.Context())
	store := s.newStore(uid)

	type params struct {
		LocationName string  `json:"location_name"`
		Lat          float64 `json:"latitude"`
		Lon          float64 `json:"longitude"`
	}
	var p params
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		slog.Debug("failed to decode request body", "error", err)
		writeError(w, http.StatusBadRequest, err)
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
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	res := toMonitorResponse(created)
	writeJSON(w, http.StatusCreated, res)
}

func (s *API) SetMonitorStatus(activate bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store := s.newStore(userID(r.Context()))

		mid, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		m, err := store.Get(r.Context(), mid)
		if errors.Is(err, monitor.ErrNotFound) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}

		if activate {
			m = m.Activate()
		} else {
			m = m.Deactivate()
		}

		updated, err := store.Update(r.Context(), m)
		if errors.Is(err, monitor.ErrNotFound) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}

		writeJSON(w, http.StatusOK, toMonitorResponse(updated))
	}
}

func (s *API) DeleteMonitor(w http.ResponseWriter, r *http.Request) {
	store := s.newStore(userID(r.Context()))

	mid, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	err = store.Delete(r.Context(), mid)
	if errors.Is(err, monitor.ErrNotFound) {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
