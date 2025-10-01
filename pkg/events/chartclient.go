package events

import (
	"context"

	"github.com/navikt/knorten/pkg/chart"
	"github.com/navikt/knorten/pkg/database"
)

type chartClient interface {
	SyncAirflow(ctx context.Context, values *chart.AirflowConfigurableValues) error
	DeleteAirflow(ctx context.Context, teamID string) error
}

type chartMock struct {
	EventCounts map[database.EventType]int
}

func newChartMock() chartMock {
	return chartMock{
		EventCounts: map[database.EventType]int{},
	}
}

func (cm chartMock) SyncAirflow(
	ctx context.Context,
	values *chart.AirflowConfigurableValues,
) error {
	cm.EventCounts[database.EventTypeCreateAirflow]++
	cm.EventCounts[database.EventTypeUpdateAirflow]++
	return nil
}

func (cm chartMock) DeleteAirflow(ctx context.Context, teamID string) error {
	cm.EventCounts[database.EventTypeDeleteAirflow]++
	return nil
}
