package events

import (
	"encoding/json"

	"github.com/nais/knorten/pkg/database/gensql"
)

func (e eventHandler) createCompute(event gensql.Event) error {
	var instance gensql.ComputeInstance
	logger := newEventLogger(e.context, e.log, e.repo, event)
	err := json.Unmarshal(event.Task, &instance)
	if err != nil {
		logger.Errorf("retrieved event with invalid param: %v", err)
		return e.setEventStatus(event.ID, gensql.EventStatusFailed)
	}

	err = e.setEventStatus(event.ID, gensql.EventStatusProcessing)
	if err != nil {
		return err
	}

	// TODO: Send med logger
	retry := e.computeClient.Create(e.context, instance)
	if retry {
		return e.setEventStatus(event.ID, gensql.EventStatusPending)
	}

	return e.setEventStatus(event.ID, gensql.EventStatusCompleted)
}

func (e eventHandler) deleteCompute(event gensql.Event) error {
	var email string
	logger := newEventLogger(e.context, e.log, e.repo, event)
	err := json.Unmarshal(event.Task, &email)
	if err != nil {
		logger.Errorf("retrieved event with invalid param: %v", err)
		return e.setEventStatus(event.ID, gensql.EventStatusFailed)
	}

	err = e.setEventStatus(event.ID, gensql.EventStatusProcessing)
	if err != nil {
		return err
	}

	retry := e.computeClient.Delete(e.context, email)
	if retry {
		return e.setEventStatus(event.ID, gensql.EventStatusPending)
	}

	return e.setEventStatus(event.ID, gensql.EventStatusCompleted)
}
