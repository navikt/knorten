package chart

import (
	"context"

	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/logger"
)

type ClientMock struct {
	EventCounts map[gensql.EventType]int
}

func NewClientMock() ClientMock {
	return ClientMock{
		EventCounts: map[gensql.EventType]int{},
	}
}

func (cm ClientMock) SyncJupyter(ctx context.Context, values JupyterConfigurableValues, log logger.Logger) bool {
	cm.EventCounts[gensql.EventTypeCreateJupyter]++
	cm.EventCounts[gensql.EventTypeUpdateJupyter]++
	return false
}

func (cm ClientMock) DeleteJupyter(ctx context.Context, teamID string, log logger.Logger) bool {
	cm.EventCounts[gensql.EventTypeDeleteJupyter]++
	return false
}

func (cm ClientMock) SyncAirflow(ctx context.Context, values AirflowConfigurableValues, log logger.Logger) bool {
	cm.EventCounts[gensql.EventTypeCreateAirflow]++
	cm.EventCounts[gensql.EventTypeUpdateAirflow]++
	return false
}

func (cm ClientMock) DeleteAirflow(ctx context.Context, teamID string, log logger.Logger) bool {
	cm.EventCounts[gensql.EventTypeDeleteAirflow]++
	return false
}
