// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.20.0

package gensql

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ChartType string

const (
	ChartTypeJupyterhub ChartType = "jupyterhub"
	ChartTypeAirflow    ChartType = "airflow"
)

func (e *ChartType) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = ChartType(s)
	case string:
		*e = ChartType(s)
	default:
		return fmt.Errorf("unsupported scan type for ChartType: %T", src)
	}
	return nil
}

type NullChartType struct {
	ChartType ChartType
	Valid     bool // Valid is true if ChartType is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullChartType) Scan(value interface{}) error {
	if value == nil {
		ns.ChartType, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.ChartType.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullChartType) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return string(ns.ChartType), nil
}

type ChartGlobalValue struct {
	ID        uuid.UUID
	Created   sql.NullTime
	Key       string
	Value     string
	ChartType ChartType
	Encrypted bool
}

type ChartTeamValue struct {
	ID        uuid.UUID
	Created   sql.NullTime
	Key       string
	Value     string
	ChartType ChartType
	TeamID    string
}

type ComputeInstance struct {
	Owner string
	Name  string
}

type Event struct {
	ID         uuid.UUID
	Type       string
	Payload    json.RawMessage
	Status     string
	Deadline   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Owner      string
	RetryCount int32
}

type EventLog struct {
	ID        uuid.UUID
	EventID   uuid.UUID
	LogType   string
	Message   string
	CreatedAt time.Time
}

type Session struct {
	Token       string
	AccessToken string
	Email       string
	Name        string
	Created     time.Time
	Expires     time.Time
	IsAdmin     bool
}

type Team struct {
	ID      string
	Slug    string
	Users   []string
	Created sql.NullTime
}

type UserGoogleSecretManager struct {
	Owner string
	Name  string
}
