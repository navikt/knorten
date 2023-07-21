package events

import (
	"encoding/json"

	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/logger"
)

func (e EventHandler) airflowEvent(event gensql.Event, logger logger.Logger) error {
	var values chart.AirflowConfigurableValues
	err := json.Unmarshal(event.Task, &values)
	if err != nil {
		return err
	}

	return e.processWork(event, values, logger)
}

func (e EventHandler) jupyterEvent(event gensql.Event, logger logger.Logger) error {
	var values chart.JupyterConfigurableValues
	err := json.Unmarshal(event.Task, &values)
	if err != nil {
		return err
	}

	return e.processWork(event, values, logger)
}
