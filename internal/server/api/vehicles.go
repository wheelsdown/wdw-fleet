package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/wheelsdown/wdw-fleet/internal/blob"
	"github.com/wheelsdown/wdw-fleet/internal/model"
	"github.com/wheelsdown/wdw-fleet/internal/store"
)

// -----------------------------------------------------------------------
// DTOs (wire types).
//
// Vehicle DTOs deliberately split from internal/model.Vehicle: the
// storage row exposes pointer-nullable columns and internal columns
// (deleted_at, custom_fields) that the wire shape shouldn't leak.
// Handlers translate via [vehicleResponseFrom].
// -----------------------------------------------------------------------

// VehicleResponse is the JSON body returned for every vehicle read
// or write. Nullable columns are pointers so the JSON literal is
// `null` rather than a zero value.
type VehicleResponse struct {
	ID                   uuid.UUID       `json:"id" openapi:"format=uuid,readOnly"`
	Name                 string          `json:"name"`
	Make                 string          `json:"make"`
	Model                string          `json:"model"`
	Year                 *int            `json:"year"`
	VIN                  string          `json:"vin"`
	LicensePlate         string          `json:"license_plate"`
	Glyph                string          `json:"glyph" openapi:"enum=truck|tractor|atv|trailer|gen|car|track|race"`
	Odometer             int             `json:"odometer"`
	OdometerUnit         string          `json:"odometer_unit" openapi:"enum=mi|km|hr|events"`
	Status               string          `json:"status" openapi:"enum=active|inactive|sold"`
	AcquisitionDate      *string         `json:"acquisition_date" openapi:"format=date"`
	AcquisitionCostCents *int            `json:"acquisition_cost_cents"`
	SaleDate             *string         `json:"sale_date" openapi:"format=date"`
	SalePriceCents       *int            `json:"sale_price_cents"`
	Notes                string          `json:"notes"`
	CustomFields         json.RawMessage `json:"custom_fields"`
	PhotoURL             string          `json:"photo_url"`
	Tags                 []string        `json:"tags"`
	CreatedAt            time.Time       `json:"created_at" openapi:"format=date-time,readOnly"`
	UpdatedAt            time.Time       `json:"updated_at" openapi:"format=date-time,readOnly"`
}

// VehicleListResponse wraps a slice of vehicles under `items` so a
// later cursor-pagination addition (adding a `next_cursor` sibling)
// isn't a breaking change.
type VehicleListResponse struct {
	Items []VehicleResponse `json:"items"`
}

// VehicleCreateInput is the request body for POST /v1/vehicles.
// Only `name` is required; all other fields fall back to database
// defaults (glyph=car, odometer=0, odometer_unit=mi, status=active).
type VehicleCreateInput struct {
	Name                 string          `json:"name"`
	Make                 string          `json:"make,omitempty"`
	Model                string          `json:"model,omitempty"`
	Year                 *int            `json:"year,omitempty"`
	VIN                  string          `json:"vin,omitempty"`
	LicensePlate         string          `json:"license_plate,omitempty"`
	Glyph                string          `json:"glyph,omitempty" openapi:"enum=truck|tractor|atv|trailer|gen|car|track|race"`
	Odometer             *int            `json:"odometer,omitempty"`
	OdometerUnit         string          `json:"odometer_unit,omitempty" openapi:"enum=mi|km|hr|events"`
	Status               string          `json:"status,omitempty" openapi:"enum=active|inactive|sold"`
	AcquisitionDate      *string         `json:"acquisition_date,omitempty" openapi:"format=date"`
	AcquisitionCostCents *int            `json:"acquisition_cost_cents,omitempty"`
	Notes                string          `json:"notes,omitempty"`
	CustomFields         json.RawMessage `json:"custom_fields,omitempty"`
	Tags                 []string        `json:"tags,omitempty"`
}

// VehicleUpdateInput is the request body for PATCH /v1/vehicles/{id}.
// All fields are optional and pointer-nullable so a caller can
// distinguish "leave unchanged" (field omitted) from "set to zero".
type VehicleUpdateInput struct {
	Name                 *string         `json:"name,omitempty"`
	Make                 *string         `json:"make,omitempty"`
	Model                *string         `json:"model,omitempty"`
	Year                 *int            `json:"year,omitempty"`
	VIN                  *string         `json:"vin,omitempty"`
	LicensePlate         *string         `json:"license_plate,omitempty"`
	Glyph                *string         `json:"glyph,omitempty" openapi:"enum=truck|tractor|atv|trailer|gen|car|track|race"`
	Odometer             *int            `json:"odometer,omitempty"`
	OdometerUnit         *string         `json:"odometer_unit,omitempty" openapi:"enum=mi|km|hr|events"`
	Status               *string         `json:"status,omitempty" openapi:"enum=active|inactive|sold"`
	AcquisitionDate      *string         `json:"acquisition_date,omitempty" openapi:"format=date"`
	AcquisitionCostCents *int            `json:"acquisition_cost_cents,omitempty"`
	SaleDate             *string         `json:"sale_date,omitempty" openapi:"format=date"`
	SalePriceCents       *int            `json:"sale_price_cents,omitempty"`
	Notes                *string         `json:"notes,omitempty"`
	CustomFields         json.RawMessage `json:"custom_fields,omitempty"`
	Tags                 *[]string       `json:"tags,omitempty"`
}

// -----------------------------------------------------------------------
// Store interface. Handlers depend on this narrow interface, not the
// concrete internal/store type; tests provide a fake.
// -----------------------------------------------------------------------

// VehicleStore is the narrow persistence contract the vehicles
// handlers require. Wired to a concrete *store.Vehicles in main;
// swapped for a fake in tests.
type VehicleStore interface {
	Create(ctx context.Context, in store.VehicleCreate) (*model.Vehicle, error)
	Get(ctx context.Context, id uuid.UUID) (*model.Vehicle, error)
	List(ctx context.Context) ([]*model.Vehicle, error)
	Update(ctx context.Context, id uuid.UUID, up store.VehicleUpdate) (*model.Vehicle, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// -----------------------------------------------------------------------
// Handlers.
// -----------------------------------------------------------------------

// maxVehiclePhotoBytes caps photo uploads. 8 MiB is comfortably
// above camera-original JPEGs after client-side resize (design
// says max 256x256 for avatars, larger for vehicle photos).
const maxVehiclePhotoBytes = 8 << 20

// vehiclePhotoBlobKey returns the blob-store key for a vehicle's
// primary photo. Extension is preserved so a browser hitting the
// serve endpoint gets a sensible Content-Type via extension mapping.
func vehiclePhotoBlobKey(id uuid.UUID, ext string) string {
	if ext == "" {
		ext = ".bin"
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return path.Join("vehicles", id.String()+ext)
}

// handleVehicleCreate implements POST /v1/vehicles.
//
// Failure modes: 400 request.bad (malformed JSON, empty name);
// 500 server.internal (store error).
func (s *Server) handleVehicleCreate(w http.ResponseWriter, r *http.Request) {
	var in VehicleCreateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		s.writeProblem(w, http.StatusBadRequest, CodeBadRequest, "malformed JSON: "+err.Error())
		return
	}
	if strings.TrimSpace(in.Name) == "" {
		s.writeProblem(w, http.StatusBadRequest, CodeValidationFailed, "name is required")
		return
	}
	acq, err := parseOptionalDate(in.AcquisitionDate)
	if err != nil {
		s.writeProblem(w, http.StatusBadRequest, CodeValidationFailed, "acquisition_date: "+err.Error())
		return
	}
	vh, err := s.Vehicles.Create(r.Context(), store.VehicleCreate{
		Name:                 in.Name,
		Make:                 in.Make,
		Model:                in.Model,
		Year:                 in.Year,
		VIN:                  in.VIN,
		LicensePlate:         in.LicensePlate,
		Glyph:                in.Glyph,
		Odometer:             in.Odometer,
		OdometerUnit:         in.OdometerUnit,
		Status:               in.Status,
		AcquisitionDate:      acq,
		AcquisitionCostCents: in.AcquisitionCostCents,
		Notes:                in.Notes,
		CustomFields:         in.CustomFields,
		Tags:                 in.Tags,
	})
	if err != nil {
		s.Logger.Error("vehicle create failed", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, CodeInternal, "could not create vehicle")
		return
	}
	s.writeJSON(w, http.StatusCreated, vehicleResponseFrom(vh))
}

// handleVehicleGet implements GET /v1/vehicles/{id}.
//
// Failure modes: 400 request.bad (unparseable UUID); 404 entity.not_found.
func (s *Server) handleVehicleGet(w http.ResponseWriter, r *http.Request) {
	id, ok := s.parseVehicleID(w, r)
	if !ok {
		return
	}
	vh, err := s.Vehicles.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		s.writeProblem(w, http.StatusNotFound, CodeNotFound, fmt.Sprintf("vehicle %s not found", id))
		return
	}
	if err != nil {
		s.Logger.Error("vehicle get failed", "id", id, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, CodeInternal, "could not fetch vehicle")
		return
	}
	s.writeJSON(w, http.StatusOK, vehicleResponseFrom(vh))
}

// handleVehicleList implements GET /v1/vehicles. Returns every
// non-deleted vehicle wrapped in `{items: [...]}`. Cursor pagination
// is a follow-up (schema and index are ready).
func (s *Server) handleVehicleList(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Vehicles.List(r.Context())
	if err != nil {
		s.Logger.Error("vehicle list failed", "error", err)
		s.writeProblem(w, http.StatusInternalServerError, CodeInternal, "could not list vehicles")
		return
	}
	items := make([]VehicleResponse, 0, len(rows))
	for _, vh := range rows {
		items = append(items, vehicleResponseFrom(vh))
	}
	s.writeJSON(w, http.StatusOK, VehicleListResponse{Items: items})
}

// handleVehicleUpdate implements PATCH /v1/vehicles/{id}. Applies a
// sparse patch; fields not present in the request body are left
// unchanged.
//
// Failure modes: 400 request.bad, 404 entity.not_found.
func (s *Server) handleVehicleUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := s.parseVehicleID(w, r)
	if !ok {
		return
	}
	var in VehicleUpdateInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		s.writeProblem(w, http.StatusBadRequest, CodeBadRequest, "malformed JSON: "+err.Error())
		return
	}
	acq, err := parseOptionalDate(in.AcquisitionDate)
	if err != nil {
		s.writeProblem(w, http.StatusBadRequest, CodeValidationFailed, "acquisition_date: "+err.Error())
		return
	}
	sale, err := parseOptionalDate(in.SaleDate)
	if err != nil {
		s.writeProblem(w, http.StatusBadRequest, CodeValidationFailed, "sale_date: "+err.Error())
		return
	}
	vh, err := s.Vehicles.Update(r.Context(), id, store.VehicleUpdate{
		Name:                 in.Name,
		Make:                 in.Make,
		Model:                in.Model,
		Year:                 in.Year,
		VIN:                  in.VIN,
		LicensePlate:         in.LicensePlate,
		Glyph:                in.Glyph,
		Odometer:             in.Odometer,
		OdometerUnit:         in.OdometerUnit,
		Status:               in.Status,
		AcquisitionDate:      acq,
		AcquisitionCostCents: in.AcquisitionCostCents,
		SaleDate:             sale,
		SalePriceCents:       in.SalePriceCents,
		Notes:                in.Notes,
		CustomFields:         in.CustomFields,
		Tags:                 in.Tags,
	})
	if errors.Is(err, store.ErrNotFound) {
		s.writeProblem(w, http.StatusNotFound, CodeNotFound, fmt.Sprintf("vehicle %s not found", id))
		return
	}
	if err != nil {
		s.Logger.Error("vehicle update failed", "id", id, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, CodeInternal, "could not update vehicle")
		return
	}
	s.writeJSON(w, http.StatusOK, vehicleResponseFrom(vh))
}

// handleVehicleDelete implements DELETE /v1/vehicles/{id}. Soft
// delete: stamps deleted_at and returns 204.
//
// Failure modes: 400 request.bad, 404 entity.not_found.
func (s *Server) handleVehicleDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := s.parseVehicleID(w, r)
	if !ok {
		return
	}
	err := s.Vehicles.Delete(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		s.writeProblem(w, http.StatusNotFound, CodeNotFound, fmt.Sprintf("vehicle %s not found", id))
		return
	}
	if err != nil {
		s.Logger.Error("vehicle delete failed", "id", id, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, CodeInternal, "could not delete vehicle")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleVehiclePhotoPut implements PUT /v1/vehicles/{id}/photo.
// Accepts a raw image body (Content-Type must be image/*), streams
// it to blob storage, and updates vehicles.photo_url to point at
// the serve endpoint. Uses PUT rather than POST for idempotency:
// replacing a photo is the primary operation.
//
// Failure modes: 400 request.bad (missing/invalid Content-Type,
// oversize body), 404 entity.not_found.
func (s *Server) handleVehiclePhotoPut(w http.ResponseWriter, r *http.Request) {
	id, ok := s.parseVehicleID(w, r)
	if !ok {
		return
	}
	// Establish the vehicle exists before writing to blob storage
	// (avoids orphaned files on 404s).
	if _, err := s.Vehicles.Get(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			s.writeProblem(w, http.StatusNotFound, CodeNotFound, fmt.Sprintf("vehicle %s not found", id))
			return
		}
		s.Logger.Error("vehicle photo put lookup failed", "id", id, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, CodeInternal, "could not verify vehicle")
		return
	}
	ct := r.Header.Get("Content-Type")
	ext, ok := photoContentTypeToExt(ct)
	if !ok {
		s.writeProblem(w, http.StatusBadRequest, CodeBadRequest,
			"unsupported Content-Type; use image/jpeg, image/png, image/webp, or image/gif")
		return
	}
	key := vehiclePhotoBlobKey(id, ext)
	// Cap the upload to prevent unbounded reads.
	limited := io.LimitReader(r.Body, maxVehiclePhotoBytes+1)
	n, err := s.Blobs.Put(r.Context(), key, limited)
	if err != nil {
		s.Logger.Error("vehicle photo blob put failed", "id", id, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, CodeInternal, "could not store photo")
		return
	}
	if n > maxVehiclePhotoBytes {
		// Undo the write; the excess was already streamed.
		_ = s.Blobs.Delete(r.Context(), key)
		s.writeProblem(w, http.StatusBadRequest, CodeBadRequest,
			fmt.Sprintf("photo exceeds %d bytes", maxVehiclePhotoBytes))
		return
	}
	serveURL := fmt.Sprintf("/v1/vehicles/%s/photo", id)
	vh, err := s.Vehicles.Update(r.Context(), id, store.VehicleUpdate{PhotoURL: &serveURL})
	if err != nil {
		s.Logger.Error("vehicle photo url update failed", "id", id, "error", err)
		s.writeProblem(w, http.StatusInternalServerError, CodeInternal, "could not link photo to vehicle")
		return
	}
	s.writeJSON(w, http.StatusOK, vehicleResponseFrom(vh))
}

// handleVehiclePhotoGet implements GET /v1/vehicles/{id}/photo.
// Streams the raw photo bytes with the recorded Content-Type
// (derived from the on-disk extension).
//
// Failure modes: 400 request.bad, 404 entity.not_found (both
// vehicle-missing and photo-missing).
func (s *Server) handleVehiclePhotoGet(w http.ResponseWriter, r *http.Request) {
	id, ok := s.parseVehicleID(w, r)
	if !ok {
		return
	}
	// Try known extensions in preference order; the store doesn't
	// currently record which extension is on disk (small
	// simplification -- vehicle_id maps to at most one photo).
	for _, ext := range photoExtensions() {
		key := vehiclePhotoBlobKey(id, ext)
		rc, err := s.Blobs.Open(r.Context(), key)
		if errors.Is(err, blob.ErrNotFound) {
			continue
		}
		if err != nil {
			s.Logger.Error("vehicle photo open failed", "id", id, "error", err)
			s.writeProblem(w, http.StatusInternalServerError, CodeInternal, "could not read photo")
			return
		}
		defer func() { _ = rc.Close() }()
		w.Header().Set("Content-Type", extToPhotoContentType(ext))
		if _, err := io.Copy(w, rc); err != nil {
			s.Logger.Debug("vehicle photo stream write failed", "id", id, "error", err)
		}
		return
	}
	s.writeProblem(w, http.StatusNotFound, CodeNotFound, fmt.Sprintf("vehicle %s has no photo", id))
}

// handleVehiclePhotoDelete implements DELETE /v1/vehicles/{id}/photo.
// Removes any stored photo and clears vehicles.photo_url. Returns
// 204 on success, 404 if the vehicle has no photo.
func (s *Server) handleVehiclePhotoDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := s.parseVehicleID(w, r)
	if !ok {
		return
	}
	found := false
	for _, ext := range photoExtensions() {
		key := vehiclePhotoBlobKey(id, ext)
		if err := s.Blobs.Delete(r.Context(), key); err == nil {
			found = true
		} else if !errors.Is(err, blob.ErrNotFound) {
			s.Logger.Error("vehicle photo delete failed", "id", id, "error", err)
			s.writeProblem(w, http.StatusInternalServerError, CodeInternal, "could not delete photo")
			return
		}
	}
	if !found {
		s.writeProblem(w, http.StatusNotFound, CodeNotFound, fmt.Sprintf("vehicle %s has no photo", id))
		return
	}
	empty := ""
	if _, err := s.Vehicles.Update(r.Context(), id, store.VehicleUpdate{PhotoURL: &empty}); err != nil {
		s.Logger.Warn("vehicle photo_url clear failed", "id", id, "error", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

// -----------------------------------------------------------------------
// Helpers.
// -----------------------------------------------------------------------

// parseVehicleID reads the {id} path parameter, validates it as a
// UUID, and writes a 400 Problem on failure. Returns (zero, false)
// on any error so the caller can bail cleanly.
func (s *Server) parseVehicleID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw := r.PathValue("id")
	id, err := uuid.Parse(raw)
	if err != nil {
		s.writeProblem(w, http.StatusBadRequest, CodeBadRequest, "invalid vehicle id: "+raw)
		return uuid.Nil, false
	}
	return id, true
}

// parseOptionalDate converts an optional "YYYY-MM-DD" wire string
// into a *time.Time. Returns (nil, nil) when the pointer is nil or
// empty; returns an error only on malformed non-empty input.
func parseOptionalDate(s *string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// formatOptionalDate is the inverse of [parseOptionalDate]. Returns
// nil (which JSON encodes as null) rather than a formatted empty
// string when the input is nil.
func formatOptionalDate(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02")
	return &s
}

// vehicleResponseFrom projects a storage-shaped model.Vehicle onto
// the wire-shaped VehicleResponse. This is the one place the DB row
// column set becomes the wire column set; adding a new column
// requires a matching decision here about whether to expose it.
func vehicleResponseFrom(v *model.Vehicle) VehicleResponse {
	cf := v.CustomFields
	if len(cf) == 0 {
		cf = json.RawMessage("{}")
	}
	tags := v.Tags
	if tags == nil {
		tags = []string{}
	}
	return VehicleResponse{
		ID:                   v.ID,
		Name:                 v.Name,
		Make:                 v.Make,
		Model:                v.Model,
		Year:                 v.Year,
		VIN:                  v.VIN,
		LicensePlate:         v.LicensePlate,
		Glyph:                v.Glyph,
		Odometer:             v.Odometer,
		OdometerUnit:         v.OdometerUnit,
		Status:               v.Status,
		AcquisitionDate:      formatOptionalDate(v.AcquisitionDate),
		AcquisitionCostCents: v.AcquisitionCostCents,
		SaleDate:             formatOptionalDate(v.SaleDate),
		SalePriceCents:       v.SalePriceCents,
		Notes:                v.Notes,
		CustomFields:         cf,
		PhotoURL:             v.PhotoURL,
		Tags:                 tags,
		CreatedAt:            v.CreatedAt,
		UpdatedAt:            v.UpdatedAt,
	}
}

// photoExtensions returns the extensions we recognize for a vehicle
// photo, in serve-preference order.
func photoExtensions() []string {
	return []string{".jpg", ".jpeg", ".png", ".webp", ".gif"}
}

// photoContentTypeToExt maps an image/* content type onto its file
// extension. Returns ("", false) for unsupported types.
func photoContentTypeToExt(ct string) (string, bool) {
	// Strip parameters ("image/jpeg; charset=binary").
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = ct[:i]
	}
	switch strings.TrimSpace(strings.ToLower(ct)) {
	case "image/jpeg", "image/jpg":
		return ".jpg", true
	case "image/png":
		return ".png", true
	case "image/webp":
		return ".webp", true
	case "image/gif":
		return ".gif", true
	}
	return "", false
}

// extToPhotoContentType is the inverse of [photoContentTypeToExt].
// Unknown extensions get application/octet-stream (should never
// happen because photos only land on disk via the PUT path).
func extToPhotoContentType(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	}
	return "application/octet-stream"
}
