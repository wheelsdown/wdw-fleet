package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestWriteProblem covers the shape and content-type of the error
// envelope: RFC 7807 media type, status echoed into the body, code
// and detail propagated, title derived from the status text.
func TestWriteProblem(t *testing.T) {
	s := &Server{Logger: newDiscardLogger()}

	rr := httptest.NewRecorder()
	s.writeProblem(rr, http.StatusNotFound, CodeNotFound, "vehicle 42 has no such id")

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", got)
	}

	var p Problem
	if err := json.Unmarshal(rr.Body.Bytes(), &p); err != nil {
		t.Fatalf("unmarshal problem body: %v", err)
	}
	if p.Status != http.StatusNotFound {
		t.Errorf("body Status = %d, want 404", p.Status)
	}
	if p.Code != CodeNotFound {
		t.Errorf("body Code = %q, want %q", p.Code, CodeNotFound)
	}
	if p.Title != http.StatusText(http.StatusNotFound) {
		t.Errorf("body Title = %q, want %q", p.Title, http.StatusText(http.StatusNotFound))
	}
	if p.Detail != "vehicle 42 has no such id" {
		t.Errorf("body Detail = %q, want it to carry the caller's detail", p.Detail)
	}
}
