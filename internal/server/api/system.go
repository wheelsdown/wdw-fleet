package api

import "net/http"

// HealthResponse is the JSON body of the /healthz liveness probe.
type HealthResponse struct {
	// Status is a fixed string ("ok") when the process is up. Kept
	// as a discriminator so an operator's monitoring can match on
	// something other than the HTTP status alone.
	Status string `json:"status"`
}

// handleHealthz answers the liveness probe. Deliberately does not
// touch the database -- a database outage is a readiness concern,
// not a liveness one, and the process should stay marked alive so
// the orchestrator does not restart-loop a healthy binary.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}
