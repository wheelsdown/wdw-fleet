package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/wheelsdown/wdw-fleet/internal/blob"
	"github.com/wheelsdown/wdw-fleet/internal/model"
	"github.com/wheelsdown/wdw-fleet/internal/store"
)

// fakeVehicleStore is an in-memory VehicleStore for handler tests.
// Not thread-safe; tests are sequential.
type fakeVehicleStore struct {
	rows       map[uuid.UUID]*model.Vehicle
	nextErr    error // one-shot error injection for the next call
	createHook func(store.VehicleCreate) *model.Vehicle
}

func newFakeVehicleStore() *fakeVehicleStore {
	return &fakeVehicleStore{rows: map[uuid.UUID]*model.Vehicle{}}
}

func (f *fakeVehicleStore) Create(_ context.Context, in store.VehicleCreate) (*model.Vehicle, error) {
	if err := f.pop(); err != nil {
		return nil, err
	}
	id := uuid.New()
	now := time.Now().UTC()
	vh := &model.Vehicle{
		ID:                   id,
		Name:                 in.Name,
		Make:                 in.Make,
		Model:                in.Model,
		Year:                 in.Year,
		VIN:                  in.VIN,
		LicensePlate:         in.LicensePlate,
		Glyph:                nonZero(in.Glyph, "car"),
		Odometer:             derefIntOr(in.Odometer, 0),
		OdometerUnit:         nonZero(in.OdometerUnit, "mi"),
		Status:               nonZero(in.Status, "active"),
		AcquisitionDate:      in.AcquisitionDate,
		AcquisitionCostCents: in.AcquisitionCostCents,
		Notes:                in.Notes,
		CustomFields:         in.CustomFields,
		Tags:                 in.Tags,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if f.createHook != nil {
		if override := f.createHook(in); override != nil {
			vh = override
		}
	}
	f.rows[id] = vh
	return vh, nil
}

func (f *fakeVehicleStore) Get(_ context.Context, id uuid.UUID) (*model.Vehicle, error) {
	if err := f.pop(); err != nil {
		return nil, err
	}
	vh, ok := f.rows[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return vh, nil
}

func (f *fakeVehicleStore) List(_ context.Context) ([]*model.Vehicle, error) {
	if err := f.pop(); err != nil {
		return nil, err
	}
	out := make([]*model.Vehicle, 0, len(f.rows))
	for _, vh := range f.rows {
		out = append(out, vh)
	}
	return out, nil
}

func (f *fakeVehicleStore) Update(_ context.Context, id uuid.UUID, up store.VehicleUpdate) (*model.Vehicle, error) {
	if err := f.pop(); err != nil {
		return nil, err
	}
	vh, ok := f.rows[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	if up.Name != nil {
		vh.Name = *up.Name
	}
	if up.Odometer != nil {
		vh.Odometer = *up.Odometer
	}
	if up.PhotoURL != nil {
		vh.PhotoURL = *up.PhotoURL
	}
	if up.Tags != nil {
		vh.Tags = *up.Tags
	}
	// Other fields omitted; extend the fake as tests need more.
	vh.UpdatedAt = time.Now().UTC()
	return vh, nil
}

func (f *fakeVehicleStore) Delete(_ context.Context, id uuid.UUID) error {
	if err := f.pop(); err != nil {
		return err
	}
	if _, ok := f.rows[id]; !ok {
		return store.ErrNotFound
	}
	delete(f.rows, id)
	return nil
}

func (f *fakeVehicleStore) pop() error {
	if e := f.nextErr; e != nil {
		f.nextErr = nil
		return e
	}
	return nil
}

func nonZero(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
func derefIntOr(p *int, fallback int) int {
	if p == nil {
		return fallback
	}
	return *p
}

// newTestServer wires a Server with a fake vehicle store and a temp
// blob store. Returns both so tests can preload / inspect state.
func newTestServer(t *testing.T) (*Server, *fakeVehicleStore) {
	t.Helper()
	fake := newFakeVehicleStore()
	return &Server{
		Logger:   newDiscardLogger(),
		Blobs:    blob.New(t.TempDir()),
		Vehicles: fake,
	}, fake
}

// ----------------------------------------------------------------------
// Core CRUD.
// ----------------------------------------------------------------------

func TestVehicleCreate(t *testing.T) {
	s, _ := newTestServer(t)
	body := `{"name":"Shop Truck","make":"Ford","model":"F-150","year":2019}`
	req := httptest.NewRequest(http.MethodPost, "/v1/vehicles", strings.NewReader(body))
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got VehicleResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Name != "Shop Truck" {
		t.Errorf("name = %q, want Shop Truck", got.Name)
	}
	if got.Glyph != "car" {
		t.Errorf("default glyph = %q, want car", got.Glyph)
	}
	if got.Status != "active" {
		t.Errorf("default status = %q, want active", got.Status)
	}
	if got.ID == uuid.Nil {
		t.Errorf("id is zero, want a generated UUID")
	}
}

func TestVehicleCreateRejectsEmptyName(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/vehicles", strings.NewReader(`{"name":"  "}`))
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var p Problem
	_ = json.Unmarshal(rr.Body.Bytes(), &p)
	if p.Code != CodeValidationFailed {
		t.Errorf("code = %q, want %q", p.Code, CodeValidationFailed)
	}
}

func TestVehicleCreateRejectsMalformedJSON(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/vehicles", strings.NewReader(`{not json`))
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestVehicleGet(t *testing.T) {
	s, fake := newTestServer(t)
	id := uuid.New()
	fake.rows[id] = &model.Vehicle{ID: id, Name: "GT3RS", Glyph: "race", Status: "active", OdometerUnit: "mi"}
	req := httptest.NewRequest(http.MethodGet, "/v1/vehicles/"+id.String(), nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	var got VehicleResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if got.Name != "GT3RS" {
		t.Errorf("name = %q, want GT3RS", got.Name)
	}
}

func TestVehicleGetNotFound(t *testing.T) {
	s, _ := newTestServer(t)
	id := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/vehicles/"+id.String(), nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestVehicleGetInvalidUUID(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/vehicles/not-a-uuid", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestVehicleList(t *testing.T) {
	s, fake := newTestServer(t)
	for i := 0; i < 3; i++ {
		id := uuid.New()
		fake.rows[id] = &model.Vehicle{ID: id, Name: "V", Glyph: "car", Status: "active", OdometerUnit: "mi"}
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/vehicles", nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	var got VehicleListResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	if len(got.Items) != 3 {
		t.Errorf("items count = %d, want 3", len(got.Items))
	}
}

func TestVehicleUpdate(t *testing.T) {
	s, fake := newTestServer(t)
	id := uuid.New()
	fake.rows[id] = &model.Vehicle{ID: id, Name: "Old", Glyph: "car", Status: "active", OdometerUnit: "mi"}
	req := httptest.NewRequest(http.MethodPatch, "/v1/vehicles/"+id.String(), strings.NewReader(`{"name":"New"}`))
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if fake.rows[id].Name != "New" {
		t.Errorf("stored name = %q, want New", fake.rows[id].Name)
	}
}

func TestVehicleDelete(t *testing.T) {
	s, fake := newTestServer(t)
	id := uuid.New()
	fake.rows[id] = &model.Vehicle{ID: id, Name: "X", Glyph: "car", Status: "active", OdometerUnit: "mi"}
	req := httptest.NewRequest(http.MethodDelete, "/v1/vehicles/"+id.String(), nil)
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rr.Code)
	}
	if _, exists := fake.rows[id]; exists {
		t.Errorf("row still present after delete")
	}
}

// ----------------------------------------------------------------------
// Photo endpoints.
// ----------------------------------------------------------------------

func TestVehiclePhotoRoundTrip(t *testing.T) {
	s, fake := newTestServer(t)
	id := uuid.New()
	fake.rows[id] = &model.Vehicle{ID: id, Name: "X", Glyph: "car", Status: "active", OdometerUnit: "mi"}

	pixels := []byte{0xff, 0xd8, 0xff, 0xd9} // minimal JPEG magic + EOI
	put := httptest.NewRequest(http.MethodPut, "/v1/vehicles/"+id.String()+"/photo", bytes.NewReader(pixels))
	put.Header.Set("Content-Type", "image/jpeg")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, put)
	if rr.Code != http.StatusOK {
		t.Fatalf("put status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var got VehicleResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &got)
	wantURL := "/v1/vehicles/" + id.String() + "/photo"
	if got.PhotoURL != wantURL {
		t.Errorf("PhotoURL = %q, want %q", got.PhotoURL, wantURL)
	}

	// GET back the bytes.
	getReq := httptest.NewRequest(http.MethodGet, "/v1/vehicles/"+id.String()+"/photo", nil)
	rr2 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr2, getReq)
	if rr2.Code != http.StatusOK {
		t.Fatalf("get status = %d", rr2.Code)
	}
	if ct := rr2.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", ct)
	}
	if !bytes.Equal(rr2.Body.Bytes(), pixels) {
		t.Errorf("photo bytes round-trip mismatch")
	}

	// DELETE.
	del := httptest.NewRequest(http.MethodDelete, "/v1/vehicles/"+id.String()+"/photo", nil)
	rr3 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr3, del)
	if rr3.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", rr3.Code)
	}

	// GET after delete -> 404.
	rr4 := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr4, httptest.NewRequest(http.MethodGet, "/v1/vehicles/"+id.String()+"/photo", nil))
	if rr4.Code != http.StatusNotFound {
		t.Errorf("post-delete get status = %d, want 404", rr4.Code)
	}
}

func TestVehiclePhotoPutRejectsUnknownContentType(t *testing.T) {
	s, fake := newTestServer(t)
	id := uuid.New()
	fake.rows[id] = &model.Vehicle{ID: id, Name: "X", Glyph: "car", Status: "active", OdometerUnit: "mi"}

	req := httptest.NewRequest(http.MethodPut, "/v1/vehicles/"+id.String()+"/photo", bytes.NewReader([]byte("junk")))
	req.Header.Set("Content-Type", "application/octet-stream")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestVehiclePhotoPutRejectsMissingVehicle(t *testing.T) {
	s, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPut, "/v1/vehicles/"+uuid.New().String()+"/photo",
		bytes.NewReader([]byte{0xff, 0xd8, 0xff, 0xd9}))
	req.Header.Set("Content-Type", "image/jpeg")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}
