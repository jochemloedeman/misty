package api

import "net/http"

func (a *API) Handler(verifier TokenVerifier) http.Handler {
	mux := http.NewServeMux()

	requireUser := RequireUser(verifier)
	protected := func(h http.HandlerFunc) http.HandlerFunc {
		return requireUser(h).ServeHTTP
	}

	mux.HandleFunc("GET /monitors", protected(a.ListMonitors))
	mux.HandleFunc("GET /monitors/{id}", protected(a.GetMonitor))
	mux.HandleFunc("GET /monitors/{id}/forecasts", protected(a.ListForecasts))
	mux.HandleFunc("POST /monitors", protected(a.CreateMonitor))
	mux.HandleFunc("POST /monitors/{id}/deactivate", protected(a.SetMonitorStatus(false)))
	mux.HandleFunc("POST /monitors/{id}/activate", protected(a.SetMonitorStatus(true)))
	mux.HandleFunc("DELETE /monitors/{id}", protected(a.DeleteMonitor))
	mux.HandleFunc("PUT /device", protected(a.UpdatePushToken))

	mux.HandleFunc("POST /register", a.Register)
	mux.HandleFunc("POST /token/refresh", a.TokenRefresh)
	mux.HandleFunc("GET /health", a.HealthCheck)

	return mux
}
