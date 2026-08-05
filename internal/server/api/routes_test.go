package api

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// TestRouteTableIntegrity locks the invariants encoded in
// [ValidateRoutes]. Runs in CI so a bad table entry fails a build
// rather than a boot.
func TestRouteTableIntegrity(t *testing.T) {
	if err := ValidateRoutes(Routes()); err != nil {
		t.Fatalf("route table failed validation: %v", err)
	}
}

// TestRouteTagsHaveDescriptions catches a tag constant that got
// referenced from a route but never added to [RouteTags].
func TestRouteTagsHaveDescriptions(t *testing.T) {
	tags := map[string]bool{}
	for _, tag := range RouteTags() {
		if tag.Name == "" {
			t.Errorf("empty tag name in RouteTags")
		}
		if tag.Description == "" {
			t.Errorf("tag %q has no description", tag.Name)
		}
		tags[tag.Name] = true
	}
	for _, rt := range Routes() {
		for _, tag := range rt.Tags {
			if !tags[tag] {
				t.Errorf("route %s %s tags %q, missing from RouteTags", rt.Method, rt.Path, tag)
			}
		}
	}
}

// TestPathParams covers the path-parameter extractor. Adjust when
// adding new mux pattern syntax.
func TestPathParams(t *testing.T) {
	cases := []struct {
		path string
		want []string
	}{
		{"/healthz", nil},
		{"/v1/vehicles", nil},
		{"/v1/vehicles/{id}", []string{"id"}},
		{"/v1/vehicles/{id}/timeline", []string{"id"}},
		{"/v1/vehicles/{vehicleId}/fuel-logs/{logId}", []string{"vehicleId", "logId"}},
		{"/v1/anything/{$}", nil},
		{"/v1/wild/{path...}", []string{"path"}},
	}
	for _, tc := range cases {
		got := PathParams(tc.path)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("PathParams(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// TestHealthzHandler is the archetypal handler test: assemble the
// full Server.Handler(), fire an httptest request, assert the shape
// of the response. No DB needed for /healthz.
func TestHealthzHandler(t *testing.T) {
	s := &Server{Logger: newDiscardLogger()}
	h := s.Handler()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("healthz Content-Type = %q, want application/json", got)
	}
	want := "{\"status\":\"ok\"}\n"
	if got := rr.Body.String(); got != want {
		t.Errorf("healthz body = %q, want %q", got, want)
	}
}
