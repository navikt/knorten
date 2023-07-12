package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/nais/knorten/pkg/database/gensql"
)

func registerEvent(ctx context.Context, eventType gensql.EventType, deadlineOffset time.Duration, form any) error {
	jsonTask, err := json.Marshal(form)
	if err != nil {
		return err
	}

	err = dbQuerier.EventCreate(ctx, gensql.EventCreateParams{
		EventType: eventType,
		Task:      jsonTask,
		Deadline:  time.Now().Add(deadlineOffset),
	})

	if err != nil {
		return err
	}

	triggerDispatcher("create team")
	return nil
}

func RegisterCreateTeamEvent(ctx context.Context, team gensql.Team) error {
	return registerEvent(ctx, gensql.EventTypeCreateTeam, 5*time.Minute, team)
}

func RegisterUpdateTeamEvent(ctx context.Context, team gensql.Team) error {
	return registerEvent(ctx, gensql.EventTypeUpdateTeam, 5*time.Minute, team)
}

func RegisterDeleteTeamEvent(ctx context.Context, team string) error {
	return registerEvent(ctx, gensql.EventTypeDeleteTeam, 5*time.Minute, team)
}
