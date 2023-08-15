package database

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nais/knorten/pkg/database/gensql"
)

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
	EventTypeCreateUserGSM EventType = "create:usergsm"
	EventTypeDeleteUserGSM EventType = "delete:usergsm"
)

type EventStatus string

const (
	EventStatusNew        EventStatus = "new"
	EventStatusProcessing EventStatus = "processing"
	EventStatusCompleted  EventStatus = "completed"
	EventStatusPending    EventStatus = "pending"
	EventStatusFailed     EventStatus = "failed"
)

type LogType string

const (
	LogTypeInfo  LogType = "info"
	LogTypeError LogType = "error"
)

type EventLog struct {
	Message   string
	LogType   LogType   `json:"log_type"`
	CreatedAt time.Time `json:"created_at"`
}

type Event struct {
	ID         uuid.UUID
	Owner      string
	Type       EventType
	Status     EventStatus
	Deadline   time.Duration
	RetryCount int32
	Payload    string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Logs       []EventLog
}

func (r *Repo) registerEvent(ctx context.Context, eventType EventType, owner string, deadlineOffset time.Duration, data any) error {
	jsonPayload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	params := gensql.EventCreateParams{
		Owner:    owner,
		Type:     string(eventType),
		Payload:  jsonPayload,
		Deadline: deadlineOffset.Milliseconds() / 1000,
	}

	if err = r.querier.EventCreate(ctx, params); err != nil {
		return err
	}

	return nil
}

func (r *Repo) RegisterCreateTeamEvent(ctx context.Context, team gensql.Team) error {
	return r.registerEvent(ctx, EventTypeCreateTeam, team.ID, 5*time.Minute, team)
}

func (r *Repo) RegisterUpdateTeamEvent(ctx context.Context, team gensql.Team) error {
	return r.registerEvent(ctx, EventTypeUpdateTeam, team.ID, 5*time.Minute, team)
}

func (r *Repo) RegisterDeleteTeamEvent(ctx context.Context, teamID string) error {
	return r.registerEvent(ctx, EventTypeDeleteTeam, teamID, 5*time.Minute, nil)
}

func (r *Repo) RegisterDeleteComputeEvent(ctx context.Context, email string) error {
	return r.registerEvent(ctx, EventTypeDeleteCompute, email, 5*time.Minute, nil)
}

func (r *Repo) RegisterCreateComputeEvent(ctx context.Context, instance gensql.ComputeInstance) error {
	return r.registerEvent(ctx, EventTypeCreateCompute, instance.Owner, 5*time.Minute, instance)
}

func (r *Repo) RegisterCreateAirflowEvent(ctx context.Context, teamID string, values any) error {
	return r.registerEvent(ctx, EventTypeCreateAirflow, teamID, 30*time.Minute, values)
}

func (r *Repo) RegisterUpdateAirflowEvent(ctx context.Context, teamID string, values any) error {
	return r.registerEvent(ctx, EventTypeUpdateAirflow, teamID, 15*time.Minute, values)
}

func (r *Repo) RegisterDeleteAirflowEvent(ctx context.Context, teamID string) error {
	return r.registerEvent(ctx, EventTypeDeleteAirflow, teamID, 5*time.Minute, nil)
}

func (r *Repo) RegisterCreateJupyterEvent(ctx context.Context, teamID string, values any) error {
	return r.registerEvent(ctx, EventTypeCreateJupyter, teamID, 5*time.Minute, values)
}

func (r *Repo) RegisterUpdateJupyterEvent(ctx context.Context, teamID string, values any) error {
	return r.registerEvent(ctx, EventTypeUpdateJupyter, teamID, 5*time.Minute, values)
}

func (r *Repo) RegisterDeleteJupyterEvent(ctx context.Context, teamID string) error {
	return r.registerEvent(ctx, EventTypeDeleteJupyter, teamID, 5*time.Minute, nil)
}

func (r *Repo) RegisterCreateUserGSMEvent(ctx context.Context, manager gensql.UserGoogleSecretManager) error {
	return r.registerEvent(ctx, EventTypeCreateUserGSM, manager.Owner, 5*time.Minute, manager)
}

func (r *Repo) RegisterDeleteUserGSMEvent(ctx context.Context, owner string) error {
	return r.registerEvent(ctx, EventTypeDeleteUserGSM, owner, 5*time.Minute, nil)
}

func (r *Repo) EventSetStatus(ctx context.Context, id uuid.UUID, status EventStatus) error {
	return r.querier.EventSetStatus(ctx, gensql.EventSetStatusParams{
		Status: string(status),
		ID:     id,
	})
}

// EventSetPendingStatus will set status to pending and increment retry_count by 1
func (r *Repo) EventSetPendingStatus(ctx context.Context, id uuid.UUID) error {
	return r.querier.EventSetPendingStatus(ctx, id)
}

func (r *Repo) DispatcherEventsGet(ctx context.Context) ([]gensql.DispatcherEventsGetRow, error) {
	return r.querier.DispatcherEventsGet(ctx)
}

func (r *Repo) EventsGetType(ctx context.Context, eventType EventType) ([]gensql.EventsGetTypeRow, error) {
	return r.querier.EventsGetType(ctx, string(eventType))
}

func (r *Repo) EventLogCreate(ctx context.Context, id uuid.UUID, message string, logType LogType) error {
	return r.querier.EventLogCreate(ctx, gensql.EventLogCreateParams{
		EventID: id,
		Message: message,
		LogType: string(logType),
	})
}

func (r *Repo) EventGet(ctx context.Context, id uuid.UUID) (Event, error) {
	eventGetRow, err := r.querier.EventGet(ctx, id)
	if err != nil {
		return Event{}, err
	}

	deadline, err := parseDeadline(eventGetRow.Deadline)
	if err != nil {
		return Event{}, err
	}

	return Event{
		ID:         eventGetRow.ID,
		Type:       EventType(eventGetRow.Type),
		Payload:    string(eventGetRow.Payload),
		Status:     EventStatus(eventGetRow.Status),
		Deadline:   deadline,
		CreatedAt:  eventGetRow.CreatedAt,
		UpdatedAt:  eventGetRow.UpdatedAt,
		Owner:      eventGetRow.Owner,
		RetryCount: eventGetRow.RetryCount,
	}, nil
}

func (r *Repo) EventsGet(ctx context.Context, teamID string, limit int32) ([]Event, error) {
	rows, err := r.querier.EventsByOwnerGet(ctx, gensql.EventsByOwnerGetParams{
		Owner: teamID,
		Lim:   limit,
	})
	if err != nil {
		return nil, err
	}

	var events []Event
	for _, row := range rows {
		deadline, err := parseDeadline(row.Deadline)
		if err != nil {
			return nil, err
		}

		event := Event{
			ID:         row.ID,
			Type:       EventType(row.Type),
			Payload:    string(row.Payload),
			Status:     EventStatus(row.Status),
			Deadline:   deadline,
			CreatedAt:  row.CreatedAt,
			UpdatedAt:  row.UpdatedAt,
			Owner:      row.Owner,
			RetryCount: row.RetryCount,
		}
		events = append(events, event)
	}

	return events, nil
}

func (r *Repo) EventLogsForEventGet(ctx context.Context, id uuid.UUID) ([]gensql.EventLogsForEventGetRow, error) {
	return r.querier.EventLogsForEventGet(ctx, id)
}

func (r *Repo) EventLogsForOwnerGet(ctx context.Context, owner string) ([]Event, error) {
	eventRows, err := r.querier.EventLogsForOwnerGet(ctx, gensql.EventLogsForOwnerGetParams{
		Owner: owner,
		Lim:   10,
	})
	if err != nil {
		return nil, err
	}

	var events []Event
	for _, row := range eventRows {
		var logs []EventLog
		err := json.Unmarshal(row.JsonLogs, &logs)
		if err != nil {
			return nil, err
		}

		deadline, err := parseDeadline(row.Deadline)
		if err != nil {
			return nil, err
		}

		events = append(events, Event{
			Owner:      row.Owner,
			Type:       EventType(row.Type),
			Status:     EventStatus(row.Status),
			Deadline:   deadline,
			RetryCount: row.RetryCount,
			CreatedAt:  row.CreatedAt,
			UpdatedAt:  row.UpdatedAt,
			Logs:       logs,
		})
	}

	return events, nil
}

func parseDeadline(input string) (time.Duration, error) {
	deadline := strings.Replace(input, ":", "h", 1)
	deadline = strings.Replace(deadline, ":", "m", 1)
	deadline += "s"

	return time.ParseDuration(deadline)
}
