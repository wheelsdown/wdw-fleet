package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Vehicle struct {
	ID              uuid.UUID       `json:"id"`
	Name            string          `json:"name"`
	Make            string          `json:"make"`
	Model           string          `json:"model"`
	Year            *int            `json:"year,omitempty"`
	VIN             string          `json:"vin"`
	LicensePlate    string          `json:"license_plate"`
	Odometer        int             `json:"odometer"`
	OdometerUnit    string          `json:"odometer_unit"`
	Status          string          `json:"status"`
	AcquisitionDate *string         `json:"acquisition_date,omitempty"`
	AcquisitionCost *int            `json:"acquisition_cost,omitempty"`
	SaleDate        *string         `json:"sale_date,omitempty"`
	SalePrice       *int            `json:"sale_price,omitempty"`
	Notes           string          `json:"notes"`
	CustomFields    json.RawMessage `json:"custom_fields"`
	PhotoURL        string          `json:"photo_url"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type FuelLog struct {
	ID         uuid.UUID `json:"id"`
	VehicleID  uuid.UUID `json:"vehicle_id"`
	Date       string    `json:"date"`
	Odometer   int       `json:"odometer"`
	Volume     float64   `json:"volume"`
	VolumeUnit string    `json:"volume_unit"`
	Cost       int       `json:"cost"`
	FullTank   bool      `json:"full_tank"`
	MissedFill bool      `json:"missed_fill"`
	Octane     string    `json:"octane"`
	Station    string    `json:"station"`
	Notes      string    `json:"notes"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ServiceRecord struct {
	ID          uuid.UUID `json:"id"`
	VehicleID   uuid.UUID `json:"vehicle_id"`
	Date        string    `json:"date"`
	Odometer    *int      `json:"odometer,omitempty"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Cost        int       `json:"cost"`
	Vendor      string    `json:"vendor"`
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Part struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	PartNumber   string    `json:"part_number"`
	Manufacturer string    `json:"manufacturer"`
	Cost         *int      `json:"cost,omitempty"`
	Vendor       string    `json:"vendor"`
	Notes        string    `json:"notes"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Expense struct {
	ID          uuid.UUID `json:"id"`
	VehicleID   uuid.UUID `json:"vehicle_id"`
	Date        string    `json:"date"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	Cost        int       `json:"cost"`
	Vendor      string    `json:"vendor"`
	Recurring   bool      `json:"recurring"`
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Reminder struct {
	ID                     uuid.UUID `json:"id"`
	VehicleID              uuid.UUID `json:"vehicle_id"`
	Name                   string    `json:"name"`
	Type                   string    `json:"type"`
	DueDate                *string   `json:"due_date,omitempty"`
	DueOdometer            *int      `json:"due_odometer,omitempty"`
	RepeatIntervalDays     *int      `json:"repeat_interval_days,omitempty"`
	RepeatIntervalDistance *int      `json:"repeat_interval_distance,omitempty"`
	Status                 string    `json:"status"`
	Notes                  string    `json:"notes"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type Webhook struct {
	ID        uuid.UUID `json:"id"`
	URL       string    `json:"url"`
	Events    []string  `json:"events"`
	Secret    string    `json:"-"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Pagination support
type PageParams struct {
	Cursor string
	Limit  int
}

type PageResult[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}
