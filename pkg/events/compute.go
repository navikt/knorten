package events

import (
	"encoding/json"

	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/logger"
)

func (e EventHandler) createCompute(event gensql.Event, logger logger.Logger) error {
	var form gensql.ComputeInstance
	err := json.Unmarshal(event.Task, &form)
	if err != nil {
		return err
	}

	return e.processWork(event, form, logger)
}
