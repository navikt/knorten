package events

import (
	"context"

	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/logger"
)

type chartClient interface {
	SyncJupyter(ctx context.Context, values chart.JupyterConfigurableValues, log logger.Logger) bool
	DeleteJupyter(ctx context.Context, teamID string, log logger.Logger) bool
	SyncAirflow(ctx context.Context, values chart.AirflowConfigurableValues, log logger.Logger) bool
	DeleteAirflow(ctx context.Context, teamID string, log logger.Logger) bool
}

type chartMock struct {
	EventCounts map[gensql.EventType]int
}

func newChartMock() chartMock {
	return chartMock{
		EventCounts: map[gensql.EventType]int{},
	}
}

func (cm chartMock) SyncJupyter(ctx context.Context, values chart.JupyterConfigurableValues, log logger.Logger) bool {
	cm.EventCounts[gensql.EventTypeCreateJupyter]++
	cm.EventCounts[gensql.EventTypeUpdateJupyter]++
	return false
}

func (cm chartMock) DeleteJupyter(ctx context.Context, teamID string, log logger.Logger) bool {
	cm.EventCounts[gensql.EventTypeDeleteJupyter]++
	return false
}

func (cm chartMock) SyncAirflow(ctx context.Context, values chart.AirflowConfigurableValues, log logger.Logger) bool {
	cm.EventCounts[gensql.EventTypeCreateAirflow]++
	cm.EventCounts[gensql.EventTypeUpdateAirflow]++
	return false
}

func (cm chartMock) DeleteAirflow(ctx context.Context, teamID string, log logger.Logger) bool {
	cm.EventCounts[gensql.EventTypeDeleteAirflow]++
	return false
}
