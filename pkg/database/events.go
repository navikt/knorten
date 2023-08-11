package database

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nais/knorten/pkg/database/gensql"
)

type EventLog struct {
	Message   string
	LogType   gensql.LogType `json:"log_type"`
	CreatedAt time.Time      `json:"created_at"`
}

type Event struct {
	ID         uuid.UUID
	Owner      string
	Type       gensql.EventType
	Status     gensql.EventStatus
	Deadline   time.Duration
	RetryCount int32
	Payload    string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Logs       []EventLog
}

func (r *Repo) registerEvent(ctx context.Context, eventType gensql.EventType, owner string, deadlineOffset time.Duration, data any) error {
	jsonPayload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	err = r.querier.EventCreate(ctx, gensql.EventCreateParams{
		Owner:     owner,
		EventType: eventType,
		Payload:   jsonPayload,
		Deadline:  deadlineOffset.Milliseconds() / 1000,
	})
	if err != nil {
		return err
	}

	return nil
}

func (r *Repo) RegisterCreateTeamEvent(ctx context.Context, team gensql.Team) error {
	return r.registerEvent(ctx, gensql.EventTypeCreateTeam, team.ID, 5*time.Minute, team)
}

func (r *Repo) RegisterUpdateTeamEvent(ctx context.Context, team gensql.Team) error {
	return r.registerEvent(ctx, gensql.EventTypeUpdateTeam, team.ID, 5*time.Minute, team)
}

func (r *Repo) RegisterDeleteTeamEvent(ctx context.Context, teamID string) error {
	return r.registerEvent(ctx, gensql.EventTypeDeleteTeam, teamID, 5*time.Minute, nil)
}

func (r *Repo) RegisterDeleteComputeEvent(ctx context.Context, email string) error {
	return r.registerEvent(ctx, gensql.EventTypeDeleteCompute, email, 5*time.Minute, nil)
}

func (r *Repo) RegisterCreateComputeEvent(ctx context.Context, instance gensql.ComputeInstance) error {
	return r.registerEvent(ctx, gensql.EventTypeCreateCompute, instance.Email, 5*time.Minute, instance)
}

func (r *Repo) RegisterCreateAirflowEvent(ctx context.Context, teamID string, values any) error {
	return r.registerEvent(ctx, gensql.EventTypeCreateAirflow, teamID, 30*time.Minute, values)
}

func (r *Repo) RegisterUpdateAirflowEvent(ctx context.Context, teamID string, values any) error {
	return r.registerEvent(ctx, gensql.EventTypeUpdateAirflow, teamID, 15*time.Minute, values)
}

func (r *Repo) RegisterDeleteAirflowEvent(ctx context.Context, teamID string) error {
	return r.registerEvent(ctx, gensql.EventTypeDeleteAirflow, teamID, 5*time.Minute, nil)
}

func (r *Repo) RegisterCreateJupyterEvent(ctx context.Context, teamID string, values any) error {
	return r.registerEvent(ctx, gensql.EventTypeCreateJupyter, teamID, 5*time.Minute, values)
}

func (r *Repo) RegisterUpdateJupyterEvent(ctx context.Context, teamID string, values any) error {
	return r.registerEvent(ctx, gensql.EventTypeUpdateJupyter, teamID, 5*time.Minute, values)
}

func (r *Repo) RegisterDeleteJupyterEvent(ctx context.Context, teamID string) error {
	return r.registerEvent(ctx, gensql.EventTypeDeleteJupyter, teamID, 5*time.Minute, nil)
}

func (r *Repo) EventSetStatus(ctx context.Context, id uuid.UUID, status gensql.EventStatus) error {
	return r.querier.EventSetStatus(ctx, gensql.EventSetStatusParams{
		Status: status,
		ID:     id,
	})
}

// EventSetPendingStatus will set status to pending and increment retry_count by 1
func (r *Repo) EventSetPendingStatus(ctx context.Context, id uuid.UUID) error {
	return r.querier.EventSetPendingStatus(ctx, id)
}

func (r *Repo) EventsGetNew(ctx context.Context) ([]gensql.Event, error) {
	rows, err := r.querier.EventsGetNew(ctx)
	if err != nil {
		return nil, err
	}

	var events []gensql.Event
	for _, row := range rows {
		event := gensql.Event{
			ID:        row.ID,
			Owner:     row.Owner,
			EventType: row.EventType,
			Payload:   row.Payload,
		}
		events = append(events, event)
	}

	return events, nil
}

func (r *Repo) EventsGetOverdue(ctx context.Context) ([]gensql.Event, error) {
	rows, err := r.querier.EventsGetOverdue(ctx)
	if err != nil {
		return nil, err
	}

	var events []gensql.Event
	for _, row := range rows {
		event := gensql.Event{
			ID:        row.ID,
			Owner:     row.Owner,
			EventType: row.EventType,
			Payload:   row.Payload,
		}
		events = append(events, event)
	}

	return events, nil
}

func (r *Repo) EventsGetType(ctx context.Context, eventType gensql.EventType) ([]gensql.EventsGetTypeRow, error) {
	return r.querier.EventsGetType(ctx, eventType)
}

func (r *Repo) EventLogCreate(ctx context.Context, id uuid.UUID, message string, logType gensql.LogType) error {
	return r.querier.EventLogCreate(ctx, gensql.EventLogCreateParams{
		EventID: id,
		Message: message,
		LogType: logType,
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
		Type:       eventGetRow.EventType,
		Payload:    string(eventGetRow.Payload),
		Status:     eventGetRow.Status,
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
			Type:       row.EventType,
			Payload:    string(row.Payload),
			Status:     row.Status,
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
			Type:       row.EventType,
			Status:     row.Status,
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
