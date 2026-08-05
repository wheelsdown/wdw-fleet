package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Problem is the shared error envelope for the wdw-fleet API, modeled
// on RFC 7807 (application/problem+json) with a machine-readable
// `code` field added so clients can branch on error class without
// pattern-matching the human-facing detail.
//
// Every handler returns a Problem on failure; every operation's
// generated OpenAPI schema references this type under the `default`
// response. Per-operation failure enumeration deliberately lives in
// handler GoDoc rather than the spec -- keeping every failure list
// in sync with reality has been the primary drift source in other
// projects, and clients that care branch on `code` regardless.
type Problem struct {
	// Type is a URI reference identifying the problem type. Uses
	// "about:blank" when no more specific URI is defined.
	Type string `json:"type,omitempty" openapi:"format=uri,description=Problem type URI."`
	// Title is a short, human-readable summary. Stable per problem
	// type; do not vary it per occurrence.
	Title string `json:"title" openapi:"description=Short human-readable summary."`
	// Status is the HTTP status code echoed into the body. Lets a
	// client that captured only the body still see the status.
	Status int `json:"status" openapi:"format=int32,description=HTTP status code."`
	// Detail is a human-readable explanation specific to this
	// occurrence. Never leaks stack traces, credentials, or internal
	// paths; safe to display to end users.
	Detail string `json:"detail,omitempty" openapi:"description=Human-readable explanation of this occurrence."`
	// Instance is a URI identifying the specific occurrence, when
	// meaningful (for example the request ID once middleware
	// assigns one). Empty for now.
	Instance string `json:"instance,omitempty" openapi:"format=uri,description=URI identifying this occurrence."`
	// Code is a stable, machine-readable identifier for the problem
	// class (for example "vehicle.not_found", "validation.failed").
	// Clients branch on this; the human-facing fields may change
	// without a contract break.
	Code string `json:"code" openapi:"description=Machine-readable error code."`
}

// Common Problem code values. Grow this list as new error classes
// appear; keep values kebab.dot.style to match the webhook event
// naming convention established in AGENTS.md.
const (
	// CodeBadRequest is the catch-all for request-parsing failures
	// (malformed JSON, missing required field, unparseable date).
	CodeBadRequest = "request.bad"
	// CodeValidationFailed is for requests that parsed but failed
	// business-rule validation (odometer decreasing, enum out of
	// range, referenced entity of the wrong kind).
	CodeValidationFailed = "validation.failed"
	// CodeUnauthorized is for requests missing a required auth
	// credential.
	CodeUnauthorized = "auth.unauthorized"
	// CodeForbidden is for requests with valid auth but insufficient
	// privilege (non-admin hitting an admin endpoint).
	CodeForbidden = "auth.forbidden"
	// CodeNotFound is for referenced entities that don't exist or
	// have been soft-deleted.
	CodeNotFound = "entity.not_found"
	// CodeConflict is for state conflicts (duplicate unique key,
	// stale write against a revised entity).
	CodeConflict = "entity.conflict"
	// CodeInternal is for unexpected server-side failures. Handlers
	// should log the underlying error and return a generic
	// message; never leak the internal cause into Detail.
	CodeInternal = "server.internal"
	// CodeServiceUnavailable is for temporary dependency outages
	// (database unreachable, IMAP down). Clients may retry.
	CodeServiceUnavailable = "server.unavailable"
)

// writeProblem emits a Problem in application/problem+json form at
// the given status. Callers pass the machine code and human-facing
// detail; Title is derived from the status and Type is left blank
// (about:blank per RFC 7807).
//
// Logs a warning if the response encoder fails -- an encoder error
// here typically means the client hung up mid-write, which is worth
// noting but not worth escalating.
func (s *Server) writeProblem(w http.ResponseWriter, status int, code, detail string) {
	p := Problem{
		Title:  http.StatusText(status),
		Status: status,
		Detail: detail,
		Code:   code,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(p); err != nil {
		s.Logger.Warn("api: problem response encode failed",
			slog.Int("status", status),
			slog.String("code", code),
			slog.String("error", err.Error()))
	}
}
