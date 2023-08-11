package chart

import (
	"context"

	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/logger"
)

type ChartClientMock struct {
	EventCounts map[gensql.EventType]int
}

func NewChartClientMock() ChartClientMock {
	return ChartClientMock{
		EventCounts: map[gensql.EventType]int{},
	}
}

func (cm ChartClientMock) SyncJupyter(ctx context.Context, values JupyterConfigurableValues, log logger.Logger) bool {
	cm.EventCounts[gensql.EventTypeCreateJupyter]++
	cm.EventCounts[gensql.EventTypeUpdateJupyter]++
	return false
}

func (cm ChartClientMock) DeleteJupyter(ctx context.Context, teamID string, log logger.Logger) bool {
	cm.EventCounts[gensql.EventTypeDeleteJupyter]++
	return false
}

func (cm ChartClientMock) SyncAirflow(ctx context.Context, values AirflowConfigurableValues, log logger.Logger) bool {
	cm.EventCounts[gensql.EventTypeCreateAirflow]++
	cm.EventCounts[gensql.EventTypeUpdateAirflow]++
	return false
}

func (cm ChartClientMock) DeleteAirflow(ctx context.Context, teamID string, log logger.Logger) bool {
	cm.EventCounts[gensql.EventTypeDeleteAirflow]++
	return false
}
