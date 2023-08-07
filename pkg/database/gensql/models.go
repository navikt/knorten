// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.16.0

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
	return ns.ChartType, nil
}

type EventStatus string

const (
	EventStatusNew        EventStatus = "new"
	EventStatusProcessing EventStatus = "processing"
	EventStatusCompleted  EventStatus = "completed"
	EventStatusPending    EventStatus = "pending"
	EventStatusFailed     EventStatus = "failed"
)

func (e *EventStatus) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = EventStatus(s)
	case string:
		*e = EventStatus(s)
	default:
		return fmt.Errorf("unsupported scan type for EventStatus: %T", src)
	}
	return nil
}

type NullEventStatus struct {
	EventStatus EventStatus
	Valid       bool // Valid is true if EventStatus is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullEventStatus) Scan(value interface{}) error {
	if value == nil {
		ns.EventStatus, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.EventStatus.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullEventStatus) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return ns.EventStatus, nil
}

type EventType string

const (
	EventTypeCreateTeam    EventType = "create:team"
	EventTypeUpdateTeam    EventType = "update:team"
	EventTypeDeleteTeam    EventType = "delete:team"
	EventTypeCreateJupyter EventType = "create:jupyter"
	EventTypeUpdateJupyter EventType = "update:jupyter"
	EventTypeDeleteJupyter EventType = "delete:jupyter"
	EventTypeCreateAirflow EventType = "create:airflow"
	EventTypeUpdateAirflow EventType = "update:airflow"
	EventTypeDeleteAirflow EventType = "delete:airflow"
	EventTypeCreateCompute EventType = "create:compute"
	EventTypeDeleteCompute EventType = "delete:compute"
)

func (e *EventType) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = EventType(s)
	case string:
		*e = EventType(s)
	default:
		return fmt.Errorf("unsupported scan type for EventType: %T", src)
	}
	return nil
}

type NullEventType struct {
	EventType EventType
	Valid     bool // Valid is true if EventType is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullEventType) Scan(value interface{}) error {
	if value == nil {
		ns.EventType, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.EventType.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullEventType) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return ns.EventType, nil
}

type LogType string

const (
	LogTypeInfo  LogType = "info"
	LogTypeError LogType = "error"
)

func (e *LogType) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = LogType(s)
	case string:
		*e = LogType(s)
	default:
		return fmt.Errorf("unsupported scan type for LogType: %T", src)
	}
	return nil
}

type NullLogType struct {
	LogType LogType
	Valid   bool // Valid is true if LogType is not NULL
}

// Scan implements the Scanner interface.
func (ns *NullLogType) Scan(value interface{}) error {
	if value == nil {
		ns.LogType, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return ns.LogType.Scan(value)
}

// Value implements the driver Valuer interface.
func (ns NullLogType) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return ns.LogType, nil
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
	Email string
	Name  string
}

type Event struct {
	ID        uuid.UUID
	EventType EventType
	Task      json.RawMessage
	Status    EventStatus
	Deadline  string
	CreatedAt time.Time
	UpdatedAt time.Time
	Owner     string
}

type EventLog struct {
	ID        uuid.UUID
	EventID   uuid.UUID
	LogType   LogType
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
	ID                    string
	Slug                  string
	Users                 []string
	Created               sql.NullTime
	PendingJupyterUpgrade bool
	PendingAirflowUpgrade bool
	RestrictAirflowEgress bool
	Owner                 string
}
