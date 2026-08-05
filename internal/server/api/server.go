package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Server is the HTTP surface's runtime instance. Constructed once in
// cmd/wdw-fleet/main.go, wired to its dependencies, and its Handler
// mounted on the process's http.Server.
type Server struct {
	// DB is the pgx connection pool. Handlers pull typed queries
	// off it directly; wdw-fleet has no ORM layer.
	DB *pgxpool.Pool
	// Logger is the shared structured logger. Handlers should log
	// through it (never fmt.Print) so operators get a single
	// coherent stream.
	Logger *slog.Logger
}

// Handler builds the http.Handler that serves the full API surface.
// Panics if the route table fails [ValidateRoutes] -- a malformed
// entry must never reach registration in a running binary.
func (s *Server) Handler() http.Handler {
	routes := Routes()
	if err := ValidateRoutes(routes); err != nil {
		panic(fmt.Errorf("api: invalid route table: %w", err))
	}
	mux := http.NewServeMux()
	for _, rt := range routes {
		mux.Handle(rt.Method+" "+rt.Path, rt.handler(s))
	}
	return mux
}

// writeJSON serializes v as JSON at status, logging any encoder error
// through the server logger. Never called with a value that lacks a
// json.Marshaler; malformed payloads are a programming error.
func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.Logger.Warn("api: response encode failed", "error", err)
	}
}
