// Package api is the HTTP surface of wdw-fleet.
//
// The single source of truth for the surface is the [Route] slice
// returned by [Routes]. [Server.Handler] registers the mux from it
// and (in a follow-up commit) the openapigen tool will project
// internal/server/api/spec/openapi.{yaml,json} from it. Every route's
// metadata (method, path, operation id, summary, tags, auth,
// request/response shapes) therefore has exactly one home; the mux
// registration and the generated contract cannot drift.
package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// AuthPosture identifies the authentication layer a route demands.
// Enforced at registration time by [Server.Handler] and projected
// into the OpenAPI operation's x-wdw-auth extension so the documented
// contract and the enforced contract derive from the same table entry.
type AuthPosture string

const (
	// AuthPublic accepts unauthenticated requests. Used for
	// /healthz, /openapi.yaml, /docs, and the setup wizard's first
	// call.
	AuthPublic AuthPosture = "public"
	// AuthSession requires a valid session cookie whose sha256 hash
	// matches a row in the sessions table.
	AuthSession AuthPosture = "session"
	// AuthAPIToken requires a bearer token whose sha256 hash matches
	// site_config.api_token_hash. Used for headless integrations
	// (Home Assistant, cron scripts).
	AuthAPIToken AuthPosture = "api_token"
	// AuthAdmin is [AuthSession] plus a users.is_admin check.
	AuthAdmin AuthPosture = "admin"
)

// Route describes one API route: the mux registration pattern, the
// OpenAPI operation metadata, the auth posture, and the request /
// response contract shapes. Every entry in the table returned by
// [Routes] is exhaustively validated by [ValidateRoutes]; malformed
// entries fail at startup and at spec-gen time.
type Route struct {
	// Method is the HTTP method the route registers under.
	Method string
	// Path is the net/http ServeMux pattern path, including any
	// {wildcard} segments (for example "/v1/vehicles/{id}").
	Path string
	// OperationID is the stable OpenAPI operationId. Client
	// generators derive method names from it; renames are
	// API-breaking.
	OperationID string
	// Summary is the one-line OpenAPI operation summary.
	Summary string
	// Tags are the OpenAPI tags grouping the operation in rendered
	// docs. Every route carries at least one.
	Tags []string
	// Auth is the authentication posture enforced for the route.
	Auth AuthPosture
	// Request is a zero value of the request-body contract type,
	// nil when the operation takes no body. The generator walks it
	// with reflection to emit the requestBody schema.
	Request any
	// RequestOptional marks a route whose handler accepts an empty
	// body (requestBody.required: false in the generated spec).
	// Only meaningful when Request is non-nil.
	RequestOptional bool
	// Response is a zero value of the success-response contract
	// type, nil when success carries no body (204).
	Response any
	// ResponseStatuses lists every status whose body carries the
	// Response schema, primary first. Statuses not listed here are
	// documented by the shared [ProblemResponse] default response.
	ResponseStatuses []int
	// handler binds the route to a server instance at registration
	// time. Unexported: tooling consumes the metadata above; only
	// [Server.Handler] needs the binding.
	handler func(*Server) http.Handler
}

// Route tag identifiers. One per domain cluster. Values become
// OpenAPI tag names, so renames reorganize the rendered sidebar and
// break deep links.
const (
	tagSystem    = "System"
	tagFleet     = "Fleet"
	tagFuel      = "Fuel"
	tagTasks     = "Tasks"
	tagParts     = "Parts"
	tagExpenses  = "Expenses"
	tagReminders = "Reminders"
	tagTrack     = "Track Events"
	tagInbox     = "Inbox"
	tagWebhooks  = "Webhooks"
	tagUsers     = "Users"
	tagSite      = "Site"
)

// ContractVersion is the version advertised as info.version of the
// generated OpenAPI document. Deliberately decoupled from the binary
// build version so the committed spec artifact doesn't churn every
// release; bump when the documented contract changes shape.
const ContractVersion = "0.1.0-draft"

// RouteTag pairs a tag with its sidebar description.
type RouteTag struct {
	Name        string
	Description string
}

// RouteTags returns every documented tag in sidebar display order.
// [ValidateRoutes] fails on a route naming a tag missing here, so a
// new domain cluster cannot ship an undescribed sidebar section.
func RouteTags() []RouteTag {
	return []RouteTag{
		{tagSystem, "Health, readiness, build metadata, and the OpenAPI contract itself."},
		{tagUsers, "User accounts, sessions, and per-user preferences."},
		{tagSite, "Site-wide configuration: branding, units, integrations, admin."},
		{tagFleet, "Vehicles: the fleet."},
		{tagFuel, "Fuel fill-ups, economy calculation, and Road Trip HD import."},
		{tagTasks, "Work log: service, repair, inspection, modification, notes."},
		{tagParts, "Parts catalog and inventory."},
		{tagExpenses, "Non-fuel monetary events (insurance, registration, tolls, ...)."},
		{tagReminders, "Date-, odometer-, and event-driven maintenance reminders."},
		{tagTrack, "Track events: HPDE, race, autocross sessions per vehicle."},
		{tagInbox, "IMAP-ingested receipts and other unfiled items awaiting routing."},
		{tagWebhooks, "Outbound event delivery configuration."},
	}
}

// Routes is the single source of truth for the HTTP surface. Handlers
// live in per-resource files (system.go, vehicles.go, ...); those
// files add their entries here.
//
// New domain: add a tag constant + [RouteTags] entry, then append the
// route entries. [ValidateRoutes] catches typos.
func Routes() []Route {
	return []Route{
		{
			Method:           http.MethodGet,
			Path:             "/healthz",
			OperationID:      "getHealth",
			Summary:          "Liveness probe. Always returns 200 if the process is up.",
			Tags:             []string{tagSystem},
			Auth:             AuthPublic,
			Response:         HealthResponse{},
			ResponseStatuses: []int{http.StatusOK},
			handler:          bindHandler((*Server).handleHealthz),
		},
	}
}

// PathParams returns the {wildcard} segment names of a ServeMux
// pattern path, in order of appearance. "{$}" is not a parameter.
func PathParams(path string) []string {
	var params []string
	for _, seg := range strings.Split(path, "/") {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(seg, "{"), "}")
			name = strings.TrimSuffix(name, "...")
			if name != "" && name != "$" {
				params = append(params, name)
			}
		}
	}
	return params
}

// ValidateRoutes rejects entries with missing, colliding, or
// incoherent metadata. The single copy of the table's invariants:
// [Server.Handler] panics on a table that fails it (a malformed entry
// must never register), the openapigen tool will refuse to emit a
// spec from one, and TestRouteTableIntegrity keeps it green in CI.
func ValidateRoutes(routes []Route) error {
	if len(routes) == 0 {
		return errors.New("route table is empty")
	}
	knownTags := map[string]bool{}
	for _, tag := range RouteTags() {
		knownTags[tag.Name] = true
	}
	validMethods := map[string]bool{
		http.MethodGet: true, http.MethodPost: true, http.MethodPut: true,
		http.MethodPatch: true, http.MethodDelete: true,
	}
	validAuth := map[AuthPosture]bool{
		AuthPublic: true, AuthSession: true, AuthAPIToken: true, AuthAdmin: true,
	}
	seenOps := map[string]bool{}
	seenPatterns := map[string]bool{}
	for _, rt := range routes {
		id := rt.Method + " " + rt.Path
		switch {
		case !validMethods[rt.Method]:
			return fmt.Errorf("route %s: unexpected method", id)
		case !strings.HasPrefix(rt.Path, "/"):
			return fmt.Errorf("route %s: path does not start with /", id)
		case rt.OperationID == "":
			return fmt.Errorf("route %s: missing OperationID", id)
		case rt.Summary == "":
			return fmt.Errorf("route %s: missing Summary", id)
		case len(rt.Tags) == 0:
			return fmt.Errorf("route %s: missing Tags", id)
		case len(rt.ResponseStatuses) == 0:
			return fmt.Errorf("route %s: missing ResponseStatuses", id)
		case rt.handler == nil:
			return fmt.Errorf("route %s: missing handler binding", id)
		case !validAuth[rt.Auth]:
			return fmt.Errorf("route %s: unknown auth posture %q", id, rt.Auth)
		case seenOps[rt.OperationID]:
			return fmt.Errorf("route %s: duplicate OperationID %q", id, rt.OperationID)
		case seenPatterns[id]:
			return fmt.Errorf("route %s: duplicate registration", id)
		case rt.RequestOptional && rt.Request == nil:
			return fmt.Errorf("route %s: RequestOptional without Request", id)
		}
		seenOps[rt.OperationID] = true
		seenPatterns[id] = true

		for _, tag := range rt.Tags {
			if !knownTags[tag] {
				return fmt.Errorf("route %s: tag %q has no RouteTags entry", id, tag)
			}
		}

		if rt.Response == nil && (len(rt.ResponseStatuses) != 1 || rt.ResponseStatuses[0] != http.StatusNoContent) {
			return fmt.Errorf("route %s: nil Response requires ResponseStatuses of exactly [204]", id)
		}
	}
	return nil
}

// bindHandler adapts a *Server method expression to the Route.handler
// shape so table entries stay one line per route.
func bindHandler(m func(*Server, http.ResponseWriter, *http.Request)) func(*Server) http.Handler {
	return func(s *Server) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { m(s, w, r) })
	}
}
