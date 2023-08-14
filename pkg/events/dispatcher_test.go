package events

import (
	"context"
	"testing"

	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/compute"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/team"
)

func TestEventHandler_distributeWork(t *testing.T) {
	checkEventType := func(eventType gensql.EventType, chartMock chart.ClientMock, teamMock team.ClientMock, computeMock compute.ClientMock) int {
		switch eventType {
		case gensql.EventTypeCreateCompute,
			gensql.EventTypeDeleteCompute:
			return computeMock.EventCounts[eventType]
		case gensql.EventTypeCreateTeam,
			gensql.EventTypeUpdateTeam,
			gensql.EventTypeDeleteTeam:
			return teamMock.EventCounts[eventType]
		case gensql.EventTypeCreateAirflow,
			gensql.EventTypeUpdateAirflow,
			gensql.EventTypeDeleteAirflow,
			gensql.EventTypeCreateJupyter,
			gensql.EventTypeUpdateJupyter,
			gensql.EventTypeDeleteJupyter:
			return chartMock.EventCounts[eventType]
		}

		return -1
	}

	eventTypes := []gensql.EventType{
		gensql.EventTypeCreateCompute,
		gensql.EventTypeDeleteCompute,
		gensql.EventTypeCreateTeam,
		gensql.EventTypeUpdateTeam,
		gensql.EventTypeDeleteTeam,
		gensql.EventTypeCreateAirflow,
		gensql.EventTypeUpdateAirflow,
		gensql.EventTypeDeleteAirflow,
		gensql.EventTypeCreateJupyter,
		gensql.EventTypeUpdateJupyter,
		gensql.EventTypeDeleteJupyter,
	}
	for _, eventType := range eventTypes {
		t.Run(string(eventType), func(t *testing.T) {
			computeMock := compute.NewClientMock()
			teamMock := team.NewClientMock()
			chartMock := chart.NewClientMock()
			handler := EventHandler{
				repo:          &database.RepoMock{},
				computeClient: &computeMock,
				teamClient:    &teamMock,
				chartClient:   &chartMock,
			}
			worker := handler.distributeWork(eventType)
			if err := worker(context.Background(), gensql.DispatcherEventsGetRow{Payload: []byte("{}"), EventType: eventType}, nil); err != nil {
				t.Errorf("worker(): %v", err)
			}

			if count := checkEventType(eventType, chartMock, teamMock, computeMock); count != 1 {
				t.Errorf("distributeWork(): expected 1 %v event, got %v", eventType, count)
			}
		})
	}
}
