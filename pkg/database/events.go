package database

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nais/knorten/pkg/database/gensql"
)

type EventWithLogs struct {
	gensql.Event
	Payload string
	Logs    []gensql.EventLog
}

func (r *Repo) registerEvent(ctx context.Context, eventType gensql.EventType, owner string, deadline time.Duration, data any) error {
	jsonPayload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	params := gensql.EventCreateParams{
		Owner:     owner,
		EventType: eventType,
		Payload:   jsonPayload,
		Deadline:  deadline.String(),
	}

	if err = r.querier.EventCreate(ctx, params); err != nil {
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

func (r *Repo) DispatchableEventsGet(ctx context.Context) ([]gensql.Event, error) {
	return r.querier.DispatchableEventsGet(ctx)
}

func (r *Repo) DispatcherEventsGet(ctx context.Context) ([]gensql.Event, error) {
	processing, err := r.querier.DispatcherEventsProcessingGet(ctx)
	if err != nil {
		return nil, err
	}
	upcoming, err := r.querier.DispatcherEventsUpcomingGet(ctx)
	if err != nil {
		return nil, err
	}

	dispatchable := []gensql.Event{}
	for _, event := range upcoming {
		if !isProcessingEventTypeForTeam(processing, event) {
			dispatchable = append(dispatchable, event)
		}
	}

	return dispatchable, nil
}

func (r *Repo) EventsGetType(ctx context.Context, eventType gensql.EventType) ([]gensql.Event, error) {
	return r.querier.EventsGetType(ctx, eventType)
}

func (r *Repo) EventLogCreate(ctx context.Context, id uuid.UUID, message string, logType gensql.LogType) error {
	return r.querier.EventLogCreate(ctx, gensql.EventLogCreateParams{
		EventID: id,
		Message: message,
		LogType: logType,
	})
}

func (r *Repo) EventGet(ctx context.Context, id uuid.UUID) (gensql.Event, error) {
	return r.querier.EventGet(ctx, id)
}

func (r *Repo) EventsByOwnerGet(ctx context.Context, teamID string, limit int32) ([]gensql.Event, error) {
	return r.querier.EventsByOwnerGet(ctx, gensql.EventsByOwnerGetParams{
		Owner: teamID,
		Lim:   limit,
	})
}

func (r *Repo) EventLogsForEventGet(ctx context.Context, id uuid.UUID) ([]gensql.EventLog, error) {
	return r.querier.EventLogsForEventGet(ctx, id)
}

func (r *Repo) EventLogsForOwnerGet(ctx context.Context, owner string) ([]EventWithLogs, error) {
	events, err := r.querier.EventsByOwnerGet(ctx, gensql.EventsByOwnerGetParams{
		Owner: owner,
		Lim:   10,
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

func isProcessingEventTypeForTeam(processing []gensql.Event, new gensql.Event) bool {
	for _, e := range processing {
		if e.Owner == new.Owner && strings.Split(string(e.EventType), ":")[1] == strings.Split(string(new.EventType), ":")[1] {
			return true
		}
	}

	return false
}
