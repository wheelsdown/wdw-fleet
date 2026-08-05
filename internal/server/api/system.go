package api

import (
	"net/http"

	"github.com/wheelsdown/wdw-fleet/internal/server/api/spec"
)

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

// handleOpenAPIYAML serves the generated OpenAPI 3.1 document as
// YAML. The bytes are embedded at build time from
// internal/server/api/spec/openapi.yaml.
func (s *Server) handleOpenAPIYAML(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	if _, err := w.Write(spec.YAML); err != nil {
		s.Logger.Debug("openapi.yaml write failed", "error", err)
	}
}

// handleOpenAPIJSON serves the generated OpenAPI 3.1 document as
// JSON. Same content as [handleOpenAPIYAML] in a different serialization.
func (s *Server) handleOpenAPIJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(spec.JSON); err != nil {
		s.Logger.Debug("openapi.json write failed", "error", err)
	}
}
