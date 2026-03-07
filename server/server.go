package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jochemloedeman/misty/monitor"
)

type contextKey string

const userIDKey contextKey = "userID"

type MonitorStore interface {
	List(ctx context.Context, userID uuid.UUID) ([]monitor.Monitor, error)
	Get(ctx context.Context, monitorID uuid.UUID) (monitor.Monitor, error)
	Create(ctx context.Context, m monitor.Monitor) (monitor.Monitor, error)
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

type Server struct {
	monitorStore MonitorStore
}

func New(monitorStore MonitorStore) *Server {
	return &Server{
		monitorStore: monitorStore,
	}
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, status int) {
	writeJSON(w, status, ErrorResponse{Error: http.StatusText(status)})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("Failed to write JSON response", "error", err)
	}
}

func (s *Server) ListMonitors(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDKey).(uuid.UUID)
	if !ok {
		writeError(w, http.StatusUnauthorized)
		return
	}

	monitors, err := s.monitorStore.List(r.Context(), userID)
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

func (s *Server) GetMonitor(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDKey).(uuid.UUID)
	if !ok {
		writeError(w, http.StatusUnauthorized)
		return
	}

	monitorID := r.PathValue("id")
	id, err := uuid.Parse(monitorID)
	if err != nil {
		writeError(w, http.StatusBadRequest)
		return
	}

	m, err := s.monitorStore.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError)
		return
	}
	if m.UserID != userID {
		writeError(w, http.StatusForbidden)
		return
	}

	res := toMonitorResponse(m)
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) CreateMonitor(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userIDKey).(uuid.UUID)
	if !ok {
		writeError(w, http.StatusUnauthorized)
		return
	}

	type params struct {
		LocationName string  `json:"location_name"`
		Lat          float64 `json:"latitude"`
		Lon          float64 `json:"longitude"`
	}
	var p params
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest)
		return
	}

	m := monitor.Monitor{
		ID:       uuid.New(),
		UserID:   userID,
		IsActive: true,
		Location: monitor.Location{
			Name: p.LocationName,
			Lat:  p.Lat,
			Lon:  p.Lon,
		},
	}
	created, err := s.monitorStore.Create(r.Context(), m)
	if err != nil {
		writeError(w, http.StatusInternalServerError)
		return
	}

	res := toMonitorResponse(created)
	writeJSON(w, http.StatusCreated, res)
}

func (s *Server) UpdateMonitor(w http.ResponseWriter, r *http.Request) {
	panic("not implemented")
}
