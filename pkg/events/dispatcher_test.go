package events

import (
	"context"
	"github.com/sirupsen/logrus"
	"testing"

	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/database/gensql"
)

func TestEventHandler_distributeWork(t *testing.T) {
	checkEventType := func(eventType database.EventType, chartMock chartMock, teamMock teamMock, computeMock userMock, helmMock helmMock) int {
		switch eventType {
		case database.EventTypeCreateCompute,
			database.EventTypeResizeCompute,
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
		case database.EventTypeHelmRolloutJupyter,
			database.EventTypeHelmRollbackJupyter,
			database.EventTypeHelmUninstallJupyter,
			database.EventTypeHelmRolloutAirflow,
			database.EventTypeHelmRollbackAirflow,
			database.EventTypeHelmUninstallAirflow:
			return helmMock.EventCounts[eventType]
		}

		return -1
	}

	eventTypes := []database.EventType{
		database.EventTypeCreateCompute,
		database.EventTypeResizeCompute,
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
		database.EventTypeHelmRolloutJupyter,
		database.EventTypeHelmRollbackJupyter,
		database.EventTypeHelmUninstallJupyter,
		database.EventTypeHelmRolloutAirflow,
		database.EventTypeHelmRollbackAirflow,
		database.EventTypeHelmUninstallAirflow,
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
			if err := worker(context.Background(), gensql.Event{Payload: []byte("{}"), Type: string(eventType)}, logrus.New()); err != nil {
				t.Errorf("worker(): %v", err)
			}

			if count := checkEventType(eventType, chartMock, teamMock, userMock, helmMock); count != 1 {
				t.Errorf("distributeWork(): expected 1 %v event, got %v", eventType, count)
			}
		})
	}
}
