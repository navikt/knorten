package events

import (
	"encoding/json"

	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/logger"
)

func (e eventHandler) createCompute(event gensql.Event, logger logger.Logger) {
	var form gensql.ComputeInstance
	err := json.Unmarshal(event.Task, &form)
	if err != nil {
		logger.Errorf("retrieved event with invalid param: %v", err)
		err = e.setEventStatus(event.ID, gensql.EventStatusFailed)
		if err != nil {
			e.log.WithError(err).Error("can't set status for event")
		}
	}

	e.processWork(event, form, logger)
}

func (e eventHandler) deleteCompute(event gensql.Event, logger logger.Logger) {
	var form string
	err := json.Unmarshal(event.Task, &form)
	if err != nil {
		logger.Errorf("retrieved event with invalid param: %v", err)
		err = e.setEventStatus(event.ID, gensql.EventStatusFailed)
		if err != nil {
			e.log.WithError(err).Error("can't set status for event")
		}
	}

	e.processWork(event, form, logger)
}
