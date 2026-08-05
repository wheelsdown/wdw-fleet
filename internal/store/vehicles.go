// Package store holds the pgx-backed persistence layer. Each domain
// gets its own file with a type wrapping the pool and methods
// implementing the interface the corresponding API-package handler
// declares locally (accept interfaces, return structs).
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wheelsdown/wdw-fleet/internal/model"
)

// ErrNotFound is returned by Get/Update/Delete when the requested
// row does not exist or has been soft-deleted. Handlers translate
// this into a 404 with code entity.not_found.
var ErrNotFound = errors.New("store: not found")

// Vehicles is the concrete vehicles-table store. Constructed once in
// main and passed to the api.Server as a VehicleStore.
type Vehicles struct {
	pool *pgxpool.Pool
}

// NewVehicles returns a Vehicles bound to the given pool.
func NewVehicles(pool *pgxpool.Pool) *Vehicles {
	return &Vehicles{pool: pool}
}

// VehicleCreate carries the fields settable at creation time. All
// fields are optional except Name; the store applies defaults for
// glyph, odometer, odometer_unit, and status from the underlying
// column defaults.
type VehicleCreate struct {
	Name                 string
	Make                 string
	Model                string
	Year                 *int
	VIN                  string
	LicensePlate         string
	Glyph                string
	Odometer             *int
	OdometerUnit         string
	Status               string
	AcquisitionDate      *time.Time
	AcquisitionCostCents *int
	Notes                string
	CustomFields         json.RawMessage
	PhotoURL             string
	Tags                 []string
}

// VehicleUpdate is a sparse patch: only non-nil fields are applied.
// Pointer semantics let PATCH distinguish "clear this to zero" from
// "leave as-is". Callers set only what changes.
type VehicleUpdate struct {
	Name                 *string
	Make                 *string
	Model                *string
	Year                 *int
	VIN                  *string
	LicensePlate         *string
	Glyph                *string
	Odometer             *int
	OdometerUnit         *string
	Status               *string
	AcquisitionDate      *time.Time
	AcquisitionCostCents *int
	SaleDate             *time.Time
	SalePriceCents       *int
	Notes                *string
	CustomFields         json.RawMessage
	PhotoURL             *string
	Tags                 *[]string
}

// Create inserts a new vehicle and returns the fully-populated row
// (with server-assigned id / created_at / updated_at / defaults).
func (v *Vehicles) Create(ctx context.Context, in VehicleCreate) (*model.Vehicle, error) {
	// Build INSERT with defaults where the caller left fields zero.
	glyph := in.Glyph
	if glyph == "" {
		glyph = "car"
	}
	unit := in.OdometerUnit
	if unit == "" {
		unit = "mi"
	}
	status := in.Status
	if status == "" {
		status = "active"
	}
	odo := 0
	if in.Odometer != nil {
		odo = *in.Odometer
	}
	custom := in.CustomFields
	if len(custom) == 0 {
		custom = json.RawMessage(`{}`)
	}
	tags := in.Tags
	if tags == nil {
		tags = []string{}
	}

	const q = `
		INSERT INTO vehicles (
			name, make, model, year, vin, license_plate, glyph,
			odometer, odometer_unit, status,
			acquisition_date, acquisition_cost_cents,
			notes, custom_fields, photo_url, tags
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10,
			$11, $12,
			$13, $14, $15, $16
		)
		RETURNING id, created_at, updated_at
	`
	var (
		id                   uuid.UUID
		createdAt, updatedAt time.Time
	)
	err := v.pool.QueryRow(ctx, q,
		in.Name, in.Make, in.Model, in.Year, in.VIN, in.LicensePlate, glyph,
		odo, unit, status,
		in.AcquisitionDate, in.AcquisitionCostCents,
		in.Notes, custom, in.PhotoURL, tags,
	).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert vehicle: %w", err)
	}
	return v.Get(ctx, id)
}

// Get returns a single non-deleted vehicle by id. Returns
// [ErrNotFound] if the vehicle doesn't exist or has been soft-deleted.
func (v *Vehicles) Get(ctx context.Context, id uuid.UUID) (*model.Vehicle, error) {
	const q = `
		SELECT id, name, make, model, year, vin, license_plate, glyph,
		       odometer, odometer_unit, status,
		       acquisition_date, acquisition_cost_cents,
		       sale_date, sale_price_cents,
		       notes, custom_fields, photo_url, tags,
		       deleted_at, created_at, updated_at
		FROM vehicles
		WHERE id = $1 AND deleted_at IS NULL
	`
	row := v.pool.QueryRow(ctx, q, id)
	vh, err := scanVehicle(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get vehicle: %w", err)
	}
	return vh, nil
}

// List returns all non-deleted vehicles ordered by created_at
// descending. Unpaginated for v1 -- fleets that outgrow this get
// cursor pagination in a follow-up (schema and index are ready).
func (v *Vehicles) List(ctx context.Context) ([]*model.Vehicle, error) {
	const q = `
		SELECT id, name, make, model, year, vin, license_plate, glyph,
		       odometer, odometer_unit, status,
		       acquisition_date, acquisition_cost_cents,
		       sale_date, sale_price_cents,
		       notes, custom_fields, photo_url, tags,
		       deleted_at, created_at, updated_at
		FROM vehicles
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
	`
	rows, err := v.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list vehicles: %w", err)
	}
	defer rows.Close()
	var out []*model.Vehicle
	for rows.Next() {
		vh, err := scanVehicle(rows)
		if err != nil {
			return nil, fmt.Errorf("scan vehicle: %w", err)
		}
		out = append(out, vh)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate vehicles: %w", err)
	}
	return out, nil
}

// Update applies a sparse patch and returns the refreshed row.
// Fields left nil in [VehicleUpdate] are unchanged. Returns
// [ErrNotFound] if the target doesn't exist or is soft-deleted.
func (v *Vehicles) Update(ctx context.Context, id uuid.UUID, up VehicleUpdate) (*model.Vehicle, error) {
	// Build SET clause dynamically; only mutate provided fields.
	sets := make([]string, 0, 20)
	args := make([]any, 0, 20)
	next := func(col string, val any) {
		args = append(args, val)
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if up.Name != nil {
		next("name", *up.Name)
	}
	if up.Make != nil {
		next("make", *up.Make)
	}
	if up.Model != nil {
		next("model", *up.Model)
	}
	if up.Year != nil {
		next("year", *up.Year)
	}
	if up.VIN != nil {
		next("vin", *up.VIN)
	}
	if up.LicensePlate != nil {
		next("license_plate", *up.LicensePlate)
	}
	if up.Glyph != nil {
		next("glyph", *up.Glyph)
	}
	if up.Odometer != nil {
		next("odometer", *up.Odometer)
	}
	if up.OdometerUnit != nil {
		next("odometer_unit", *up.OdometerUnit)
	}
	if up.Status != nil {
		next("status", *up.Status)
	}
	if up.AcquisitionDate != nil {
		next("acquisition_date", *up.AcquisitionDate)
	}
	if up.AcquisitionCostCents != nil {
		next("acquisition_cost_cents", *up.AcquisitionCostCents)
	}
	if up.SaleDate != nil {
		next("sale_date", *up.SaleDate)
	}
	if up.SalePriceCents != nil {
		next("sale_price_cents", *up.SalePriceCents)
	}
	if up.Notes != nil {
		next("notes", *up.Notes)
	}
	if len(up.CustomFields) > 0 {
		next("custom_fields", up.CustomFields)
	}
	if up.PhotoURL != nil {
		next("photo_url", *up.PhotoURL)
	}
	if up.Tags != nil {
		next("tags", *up.Tags)
	}
	if len(sets) == 0 {
		// Empty patch: return current row unchanged.
		return v.Get(ctx, id)
	}
	args = append(args, id)
	q := fmt.Sprintf(`
		UPDATE vehicles
		SET %s
		WHERE id = $%d AND deleted_at IS NULL
	`, joinComma(sets), len(args))
	tag, err := v.pool.Exec(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("update vehicle: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return v.Get(ctx, id)
}

// Delete soft-deletes a vehicle by stamping deleted_at. Returns
// [ErrNotFound] if the target doesn't exist or is already deleted.
func (v *Vehicles) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE vehicles SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	tag, err := v.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete vehicle: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// scanVehicle reads a full vehicle row from a pgx.Row or a
// pgx.Rows (both satisfy the narrow local interface via Scan).
type scannable interface {
	Scan(dest ...any) error
}

func scanVehicle(row scannable) (*model.Vehicle, error) {
	vh := &model.Vehicle{}
	err := row.Scan(
		&vh.ID, &vh.Name, &vh.Make, &vh.Model, &vh.Year, &vh.VIN, &vh.LicensePlate, &vh.Glyph,
		&vh.Odometer, &vh.OdometerUnit, &vh.Status,
		&vh.AcquisitionDate, &vh.AcquisitionCostCents,
		&vh.SaleDate, &vh.SalePriceCents,
		&vh.Notes, &vh.CustomFields, &vh.PhotoURL, &vh.Tags,
		&vh.DeletedAt, &vh.CreatedAt, &vh.UpdatedAt,
	)
	return vh, err
}

// joinComma is strings.Join with ", " -- inlined to avoid importing
// strings for a single call.
func joinComma(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	n := len(parts) - 1
	for _, p := range parts {
		n += len(p)
	}
	out := make([]byte, 0, n)
	for i, p := range parts {
		if i > 0 {
			out = append(out, ',', ' ')
		}
		out = append(out, p...)
	}
	return string(out)
}
