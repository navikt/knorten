package events

import (
	"encoding/json"

	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/logger"
)

func (e EventHandler) createTeam(event gensql.Event, logger logger.Logger) error {
	var form gensql.Team
	err := json.Unmarshal(event.Task, &form)
	if err != nil {
		return err
	}

	e.processWork(event, form, logger)

	return nil
}

func (e EventHandler) updateTeam(event gensql.Event, logger logger.Logger) error {
	var form gensql.Team
	err := json.Unmarshal(event.Task, &form)
	if err != nil {
		return err
	}

	e.processWork(event, form, logger)

	return nil
}
