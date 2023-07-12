package events

import (
	"context"
	"encoding/json"

	"github.com/nais/knorten/pkg/database/gensql"
)

func createTeam(ctx context.Context, event gensql.Event) error {
	var form gensql.Team
	logger := newEventLogger(event)
	err := json.Unmarshal(event.Task, &form)
	if err != nil {
		logger.Errorf("retrieved event with invalid param: %v", err)
		return setEventStatus(event.ID, gensql.EventStatusFailed)
	}

	err = setEventStatus(event.ID, gensql.EventStatusProcessing)
	if err != nil {
		return err
	}

	retry := teamClient.Create(ctx, form, logger)
	if retry {
		return setEventStatus(event.ID, gensql.EventStatusPending)
	}

	return setEventStatus(event.ID, gensql.EventStatusCompleted)
}

func updateTeam(ctx context.Context, event gensql.Event) gensql.EventStatus {
	var form gensql.Team
	logger := newEventLogger(event)
	err := json.Unmarshal(event.Task, &form)
	if err != nil {
		logger.Errorf("retrieved event with invalid param: %v", err)
		return gensql.EventStatusFailed
	}

	err = setEventStatus(event.ID, gensql.EventStatusProcessing)
	if err != nil {
		logger.Errorf("can't change status, trying again soon")
		return gensql.EventStatusPending
	}

	retry := teamClient.Update(ctx, form, logger)
	if retry {
		return gensql.EventStatusPending
	}

	return gensql.EventStatusCompleted
}

func deleteTeam(ctx context.Context, event gensql.Event) error {
	var team string
	logger := newEventLogger(event)
	err := json.Unmarshal(event.Task, &team)
	if err != nil {
		logger.Errorf("retrieved event with invalid param: %v", err)
		return setEventStatus(event.ID, gensql.EventStatusFailed)
	}

	err = setEventStatus(event.ID, gensql.EventStatusPending)
	if err != nil {
		return err
	}

	retry := teamClient.Delete(ctx, team, logger)
	if retry {
		return setEventStatus(event.ID, gensql.EventStatusPending)
	}

	return setEventStatus(event.ID, gensql.EventStatusCompleted)
}
