package database

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/events"
)

func (r *Repo) registerEvent(ctx context.Context, eventType gensql.EventType, deadlineOffset time.Duration, data any) error {
	jsonTask, err := json.Marshal(data)
	if err != nil {
		return err
	}

	err = r.querier.EventCreate(ctx, gensql.EventCreateParams{
		EventType: eventType,
		Task:      jsonTask,
		Deadline:  time.Now().Add(deadlineOffset),
	})
	if err != nil {
		return err
	}

	events.TriggerDispatcher(eventType)

	return nil
}

func (r *Repo) RegisterCreateTeamEvent(ctx context.Context, team gensql.Team) error {
	return r.registerEvent(ctx, gensql.EventTypeCreateTeam, 5*time.Minute, team)
}

func (r *Repo) RegisterUpdateTeamEvent(ctx context.Context, team gensql.Team) error {
	return r.registerEvent(ctx, gensql.EventTypeUpdateTeam, 5*time.Minute, team)
}

func (r *Repo) RegisterDeleteTeamEvent(ctx context.Context, team string) error {
	return r.registerEvent(ctx, gensql.EventTypeDeleteTeam, 5*time.Minute, team)
}

func (r *Repo) EventLogCreate(ctx context.Context, id uuid.UUID, message string, logType gensql.LogType) error {
	return r.querier.EventLogCreate(ctx, gensql.EventLogCreateParams{
		EventID: id,
		Message: message,
		LogType: logType,
	})
}

func (r *Repo) EventSetDeadline(ctx context.Context, deadline time.Time) error {
	return r.querier.EventSetDeadline(ctx, gensql.EventSetDeadlineParams{Deadline: deadline})
}
func (r *Repo) EventSetStatus(ctx context.Context, id uuid.UUID, status gensql.EventStatus) error {
	return r.querier.EventSetStatus(ctx, gensql.EventSetStatusParams{
		Status: status,
		ID:     id,
	})
}

func (r *Repo) EventsGetNew(ctx context.Context) ([]gensql.Event, error) {
	return r.querier.EventsGetNew(ctx)
}

func (r *Repo) EventsGetOverdue(ctx context.Context) ([]gensql.Event, error) {
	return r.querier.EventsGetOverdue(ctx)
}

func (r *Repo) EventsGetPending(ctx context.Context) ([]gensql.Event, error) {
	return r.querier.EventsGetPending(ctx)
}
