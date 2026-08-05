package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Vehicle is the Go mirror of a row in the vehicles table. It is
// intentionally storage-shaped rather than wire-shaped: nullable DB
// columns are pointer fields, JSONB is json.RawMessage, tags are a
// slice. Wire projections (internal/server/api/vehicles.go) map this
// onto the caller-facing DTO.
type Vehicle struct {
	ID                   uuid.UUID
	Name                 string
	Make                 string
	Model                string
	Year                 *int
	VIN                  string
	LicensePlate         string
	Glyph                string
	Odometer             int
	OdometerUnit         string
	Status               string
	AcquisitionDate      *time.Time
	AcquisitionCostCents *int
	SaleDate             *time.Time
	SalePriceCents       *int
	Notes                string
	CustomFields         json.RawMessage
	PhotoURL             string
	Tags                 []string
	DeletedAt            *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}
