package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/nais/knorten/pkg/database/gensql"
)

func registerEvent(ctx context.Context, eventType gensql.EventType, deadlineOffset time.Duration, form interface{}) error {
	jsonTask, err := json.Marshal(form)
	if err != nil {
		return err
	}

	err = dbQuerier.EventCreate(ctx, gensql.EventCreateParams{
		EventType: eventType,
		Task:      jsonTask,
		Duration:  time.Time{}.Add(deadlineOffset),
	})

	if err != nil {
		return err
	}

	triggerDispatcher("create team")
	return nil
}

func RegisterCreateTeamEvent(ctx context.Context, form interface{}) error {
	return registerEvent(ctx, gensql.EventTypeCreateTeam, 5*time.Minute, form)
}
