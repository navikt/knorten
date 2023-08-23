package events

import (
	"context"
	"testing"

	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
)

func TestEventHandler_distributeWork(t *testing.T) {
	checkEventType := func(eventType database.EventType, chartMock chartMock, teamMock teamMock, computeMock userMock, helmMock helmMock) int {
		switch eventType {
		case database.EventTypeCreateCompute,
			database.EventTypeDeleteCompute:
			return computeMock.EventCounts[eventType]
		case database.EventTypeCreateTeam,
			database.EventTypeUpdateTeam,
			database.EventTypeDeleteTeam:
			return teamMock.EventCounts[eventType]
		case database.EventTypeCreateAirflow,
			database.EventTypeUpdateAirflow,
			database.EventTypeDeleteAirflow,
			database.EventTypeCreateJupyter,
			database.EventTypeUpdateJupyter,
			database.EventTypeDeleteJupyter:
			return chartMock.EventCounts[eventType]
		case database.EventTypeHelmInstallOrUpgrade,
			database.EventTypeHelmRollback,
			database.EventTypeHelmUninstall:
			return helmMock.EventCounts[eventType]
		}

		return -1
	}

	eventTypes := []database.EventType{
		database.EventTypeCreateCompute,
		database.EventTypeDeleteCompute,
		database.EventTypeCreateTeam,
		database.EventTypeUpdateTeam,
		database.EventTypeDeleteTeam,
		database.EventTypeCreateAirflow,
		database.EventTypeUpdateAirflow,
		database.EventTypeDeleteAirflow,
		database.EventTypeCreateJupyter,
		database.EventTypeUpdateJupyter,
		database.EventTypeDeleteJupyter,
		database.EventTypeHelmInstallOrUpgrade,
		database.EventTypeHelmRollback,
		database.EventTypeHelmUninstall,
	}
	for _, eventType := range eventTypes {
		t.Run(string(eventType), func(t *testing.T) {
			userMock := newUserMock()
			teamMock := newTeamMock()
			chartMock := newChartMock()
			helmMock := newHelmMock()
			handler := EventHandler{
				repo:        &database.RepoMock{},
				userClient:  &userMock,
				teamClient:  &teamMock,
				chartClient: &chartMock,
				helmClient:  &helmMock,
			}
			worker := handler.distributeWork(eventType)
			if err := worker(context.Background(), gensql.Event{Payload: []byte("{}"), Type: string(eventType)}, nil); err != nil {
				t.Errorf("worker(): %v", err)
			}

			if count := checkEventType(eventType, chartMock, teamMock, userMock, helmMock); count != 1 {
				t.Errorf("distributeWork(): expected 1 %v event, got %v", eventType, count)
			}
		})
	}
}
