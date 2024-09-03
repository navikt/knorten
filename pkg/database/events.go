package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/navikt/knorten/pkg/database/gensql"
)

type EventType string

const (
	EventTypeCreateTeam           EventType = "create:team"
	EventTypeUpdateTeam           EventType = "update:team"
	EventTypeDeleteTeam           EventType = "delete:team"
	EventTypeCreateJupyter        EventType = "create:jupyter"
	EventTypeUpdateJupyter        EventType = "update:jupyter"
	EventTypeDeleteJupyter        EventType = "delete:jupyter"
	EventTypeCreateAirflow        EventType = "create:airflow"
	EventTypeUpdateAirflow        EventType = "update:airflow"
	EventTypeDeleteAirflow        EventType = "delete:airflow"
	EventTypeCreateCompute        EventType = "create:compute"
	EventTypeResizeCompute        EventType = "resize:compute"
	EventTypeDeleteCompute        EventType = "delete:compute"
	EventTypeCreateUserGSM        EventType = "create:usergsm"
	EventTypeDeleteUserGSM        EventType = "delete:usergsm"
	EventTypeHelmRolloutJupyter   EventType = "rolloutJupyter:helm"
	EventTypeHelmRollbackJupyter  EventType = "rollbackJupyter:helm"
	EventTypeHelmUninstallJupyter EventType = "uninstallJupyter:helm"
	EventTypeHelmRolloutAirflow   EventType = "rolloutAirflow:helm"
	EventTypeHelmRollbackAirflow  EventType = "rollbackAirflow:helm"
	EventTypeHelmUninstallAirflow EventType = "uninstallAirflow:helm"
)

type EventStatus string

const (
	EventStatusNew             EventStatus = "new"
	EventStatusProcessing      EventStatus = "processing"
	EventStatusCompleted       EventStatus = "completed"
	EventStatusPending         EventStatus = "pending"
	EventStatusFailed          EventStatus = "failed"
	EventStatusManualFailed    EventStatus = "manual_failed"
	EventStatusDeadlineReached EventStatus = "deadline_reached"
)

type LogType string

const (
	LogTypeInfo  LogType = "info"
	LogTypeError LogType = "error"
)

type EventWithLogs struct {
	gensql.Event
	Payload string
	Logs    []gensql.EventLog
}

func (r *Repo) registerEvent(ctx context.Context, eventType EventType, owner string, deadline time.Duration, data any) error {
	jsonPayload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	params := gensql.EventCreateParams{
		Owner:    owner,
		Type:     string(eventType),
		Payload:  jsonPayload,
		Deadline: deadline.String(),
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

func (r *Repo) RegisterCreateComputeEvent(ctx context.Context, owner string, values any) error {
	return r.registerEvent(ctx, EventTypeCreateCompute, owner, 5*time.Minute, values)
}

func (r *Repo) RegisterResizeComputeDiskEvent(ctx context.Context, owner string, values any) error {
	return r.registerEvent(ctx, EventTypeResizeCompute, owner, 5*time.Minute, values)
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

func (r *Repo) RegisterCreateUserGSMEvent(ctx context.Context, owner string, values any) error {
	return r.registerEvent(ctx, EventTypeCreateUserGSM, owner, 5*time.Minute, values)
}

func (r *Repo) RegisterDeleteUserGSMEvent(ctx context.Context, owner string) error {
	return r.registerEvent(ctx, EventTypeDeleteUserGSM, owner, 5*time.Minute, nil)
}

func (r *Repo) RegisterHelmRolloutJupyterEvent(ctx context.Context, teamID string, values any) error {
	return r.registerEvent(ctx, EventTypeHelmRolloutJupyter, teamID, 10*time.Minute, values)
}

func (r *Repo) RegisterHelmRollbackJupyterEvent(ctx context.Context, teamID string, values any) error {
	return r.registerEvent(ctx, EventTypeHelmRollbackJupyter, teamID, 5*time.Minute, values)
}

func (r *Repo) RegisterHelmUninstallJupyterEvent(ctx context.Context, teamID string, values any) error {
	return r.registerEvent(ctx, EventTypeHelmUninstallJupyter, teamID, 10*time.Minute, values)
}

func (r *Repo) RegisterHelmRolloutAirflowEvent(ctx context.Context, teamID string, values any) error {
	return r.registerEvent(ctx, EventTypeHelmRolloutAirflow, teamID, 30*time.Minute, values)
}

func (r *Repo) RegisterHelmRollbackAirflowEvent(ctx context.Context, teamID string, values any) error {
	return r.registerEvent(ctx, EventTypeHelmRollbackAirflow, teamID, 5*time.Minute, values)
}

func (r *Repo) RegisterHelmUninstallAirflowEvent(ctx context.Context, teamID string, values any) error {
	return r.registerEvent(ctx, EventTypeHelmUninstallAirflow, teamID, 10*time.Minute, values)
}

func (r *Repo) EventSetStatus(ctx context.Context, id uuid.UUID, status EventStatus) error {
	return r.querier.EventSetStatus(ctx, gensql.EventSetStatusParams{
		Status: string(status),
		ID:     id,
	})
}

func (r *Repo) EventIncrementRetryCount(ctx context.Context, id uuid.UUID) error {
	return r.querier.EventIncrementRetryCount(ctx, id)
}

func (r *Repo) EventsReset(ctx context.Context) error {
	return r.querier.EventsReset(ctx)
}

func (r *Repo) DispatchableEventsGet(ctx context.Context) ([]gensql.Event, error) {
	processingEvents, err := r.querier.EventsProcessingGet(ctx)
	if err != nil {
		return nil, err
	}

	upcomingEvents, err := r.querier.EventsUpcomingGet(ctx)
	if err != nil {
		return nil, err
	}

	var dispatchableEvents []gensql.Event
	for _, upcomingEvent := range upcomingEvents {
		if isEventDispatchable(processingEvents, dispatchableEvents, upcomingEvent) {
			dispatchableEvents = append(dispatchableEvents, upcomingEvent)
		}
	}

	return dispatchableEvents, nil
}

func isEventDispatchable(processingEvents, dispatchableEvents []gensql.Event, upcoming gensql.Event) bool {
	if containsEvent(dispatchableEvents, upcoming) {
		return false
	}
	if containsEvent(processingEvents, upcoming) {
		return false
	}

	return true
}

func containsEvent(events []gensql.Event, new gensql.Event) bool {
	for _, event := range events {
		eventType := strings.Split(string(event.Type), ":")[1]
		newType := strings.Split(string(new.Type), ":")[1]
		if event.Owner == new.Owner && eventType == newType {
			return true
		}
	}

	return false
}

func (r *Repo) EventsGetType(ctx context.Context, eventType EventType) ([]gensql.Event, error) {
	return r.querier.EventsGetType(ctx, string(eventType))
}

func (r *Repo) EventLogCreate(ctx context.Context, id uuid.UUID, message string, logType LogType) error {
	return r.querier.EventLogCreate(ctx, gensql.EventLogCreateParams{
		EventID: id,
		Message: message,
		LogType: string(logType),
	})
}

func (r *Repo) EventGet(ctx context.Context, id uuid.UUID) (gensql.Event, error) {
	return r.querier.EventGet(ctx, id)
}

func (r *Repo) EventsByOwnerGet(ctx context.Context, teamID string, limit int32) ([]gensql.Event, error) {
	return r.querier.EventsByOwnerGet(ctx, gensql.EventsByOwnerGetParams{
		Owner: teamID,
		Lim:   sql.NullInt32{Int32: limit, Valid: limit > 0},
	})
}

func (r *Repo) EventLogsForEventGet(ctx context.Context, id uuid.UUID) ([]gensql.EventLog, error) {
	return r.querier.EventLogsForEventGet(ctx, id)
}

func (r *Repo) EventLogsForOwnerGet(ctx context.Context, owner string, limit int32) ([]EventWithLogs, error) {
	events, err := r.querier.EventsByOwnerGet(ctx, gensql.EventsByOwnerGetParams{
		Owner: owner,
		Lim:   sql.NullInt32{Int32: limit, Valid: limit > 0},
	})

	eventsWithLogs := make([]EventWithLogs, len(events))
	for i, event := range events {
		eventslogs, err := r.querier.EventLogsForEventGet(ctx, event.ID)
		if err != nil {
			return nil, err
		}

		eventsWithLogs[i] = EventWithLogs{
			Event: event,
			Logs:  eventslogs,
		}
	}

	return eventsWithLogs, err
}
