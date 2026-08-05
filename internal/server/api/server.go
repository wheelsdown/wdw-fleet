package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wheelsdown/wdw-fleet/internal/blob"
)

// Server is the HTTP surface's runtime instance. Constructed once in
// cmd/wdw-fleet/main.go, wired to its dependencies, and its Handler
// mounted on the process's http.Server.
type Server struct {
	// DB is the pgx connection pool. Kept on the Server for handler
	// paths that do ad-hoc queries; domain persistence generally
	// goes through a store interface (Vehicles, etc.) so handler
	// tests can pass a fake.
	DB *pgxpool.Pool
	// Logger is the shared structured logger. Handlers should log
	// through it (never fmt.Print) so operators get a single
	// coherent stream.
	Logger *slog.Logger
	// Blobs persists opaque payloads (vehicle photos, attachments,
	// IMAP raw messages). Currently a local-filesystem impl; the
	// package's narrow surface leaves room for an S3-compatible
	// swap later.
	Blobs *blob.Local
	// Vehicles is the vehicles-table persistence contract. Concrete
	// impl is *store.Vehicles in prod; fake in tests.
	Vehicles VehicleStore
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
