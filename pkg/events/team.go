package events

import (
	"encoding/json"

	"github.com/nais/knorten/pkg/database/gensql"
)

func (e eventHandler) createTeam(event gensql.Event) error {
	var form gensql.Team
	logger := newEventLogger(e.context, e.log, e.repo, event)
	err := json.Unmarshal(event.Task, &form)
	if err != nil {
		logger.Errorf("retrieved event with invalid param: %v", err)
		return e.setEventStatus(event.ID, gensql.EventStatusFailed)
	}

	err = e.setEventStatus(event.ID, gensql.EventStatusProcessing)
	if err != nil {
		return err
	}

	retry := e.teamClient.Create(e.context, form, logger)
	if retry {
		return e.setEventStatus(event.ID, gensql.EventStatusPending)
	}

	return e.setEventStatus(event.ID, gensql.EventStatusCompleted)
}

func (e eventHandler) updateTeam(event gensql.Event) gensql.EventStatus {
	var form gensql.Team
	logger := newEventLogger(e.context, e.log, e.repo, event)
	err := json.Unmarshal(event.Task, &form)
	if err != nil {
		logger.Errorf("retrieved event with invalid param: %v", err)
		return gensql.EventStatusFailed
	}

	err = e.setEventStatus(event.ID, gensql.EventStatusProcessing)
	if err != nil {
		logger.Errorf("can't change status, trying again soon")
		return gensql.EventStatusPending
	}

	retry := e.teamClient.Update(e.context, form, logger)
	if retry {
		return gensql.EventStatusPending
	}

	return gensql.EventStatusCompleted
}

func (e eventHandler) deleteTeam(event gensql.Event) error {
	var team string
	logger := newEventLogger(e.context, e.log, e.repo, event)
	err := json.Unmarshal(event.Task, &team)
	if err != nil {
		logger.Errorf("retrieved event with invalid param: %v", err)
		return e.setEventStatus(event.ID, gensql.EventStatusFailed)
	}

	err = e.setEventStatus(event.ID, gensql.EventStatusPending)
	if err != nil {
		return err
	}

	retry := e.teamClient.Delete(e.context, team, logger)
	if retry {
		return e.setEventStatus(event.ID, gensql.EventStatusPending)
	}

	return e.setEventStatus(event.ID, gensql.EventStatusCompleted)
}
