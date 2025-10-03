package events

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/database/gensql"
	"github.com/navikt/knorten/pkg/maintenance"
)

func TestEventHandler_distributeWork(t *testing.T) {
	checkEventType := func(eventType database.EventType, chartMock chartMock, teamMock teamMock, userMock userMock, helmMock helmMock, airflowMock airflowMock) int {
		switch eventType {
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
		case database.EventTypeDeleteSchedulerPods:
			return airflowMock.EventCounts[eventType]
		}

		return -1
	}

	eventTypes := []database.EventType{
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
		database.EventTypeDeleteSchedulerPods,
	}
	for _, eventType := range eventTypes {
		t.Run(string(eventType), func(t *testing.T) {
			userMock := newUserMock()
			teamMock := newTeamMock()
			chartMock := newChartMock()
			helmMock := newHelmMock()
			airflowMock := newAirflowMock()
			handler := EventHandler{
				repo:          &database.RepoMock{},
				userClient:    &userMock,
				teamClient:    &teamMock,
				chartClient:   &chartMock,
				helmClient:    &helmMock,
				airflowClient: &airflowMock,
			}
			worker := handler.distributeWork(eventType)
			if err := worker(context.Background(), gensql.Event{Payload: []byte("{}"), Type: string(eventType)}, logrus.New()); err != nil {
				t.Errorf("worker(): %v", err)
			}

			if count := checkEventType(eventType, chartMock, teamMock, userMock, helmMock, airflowMock); count != 1 {
				t.Errorf("distributeWork(): expected 1 %v event, got %v", eventType, count)
			}
		})
	}
}

func TestOmitAirflowEventsIfUpgradesPaused(t *testing.T) {
	teamOneID := "teamone-1234"
	teamTwoID := "teamtwo-4321"

	maintenanceExclusionConfig := &maintenance.MaintenanceExclusion{Periods: map[string][]*maintenance.MaintenanceExclusionPeriod{
		teamOneID: {
			{
				Name:  "active period for team one",
				Team:  teamOneID,
				Start: time.Now(),
				End:   time.Now().Add(time.Hour * 24),
			},
		},
		teamTwoID: {
			{
				Name:  "active period for team two",
				Team:  teamTwoID,
				Start: time.Now().Add(time.Hour * 24),
				End:   time.Now().Add(time.Hour * 48),
			},
		},
	}}

	airflowEventsOmittedTests := []struct {
		name     string
		events   []gensql.Event
		expected []gensql.Event
	}{
		{
			name: "airflow events are omitted",
			events: []gensql.Event{
				{
					Status: string(database.EventStatusNew),
					Type:   string(database.EventTypeUpdateJupyter),
					Owner:  teamOneID,
				},
				{
					Status: string(database.EventStatusNew),
					Type:   string(database.EventTypeCreateAirflow),
					Owner:  teamOneID,
				},
				{
					Status: string(database.EventStatusNew),
					Type:   string(database.EventTypeUpdateAirflow),
					Owner:  teamOneID,
				},
				{
					Status: string(database.EventStatusNew),
					Type:   string(database.EventTypeHelmRolloutAirflow),
					Owner:  teamOneID,
				},
				{
					Status: string(database.EventStatusNew),
					Type:   string(database.EventTypeHelmRollbackAirflow),
					Owner:  teamOneID,
				},
			},
			expected: []gensql.Event{
				{
					Status: string(database.EventStatusNew),
					Type:   string(database.EventTypeUpdateJupyter),
					Owner:  teamOneID,
				},
			},
		},
		{
			name: "airflow event is only omitted for team affected by maintenance exclusion",
			events: []gensql.Event{
				{
					Status: string(database.EventStatusNew),
					Type:   string(database.EventTypeUpdateJupyter),
					Owner:  teamOneID,
				},
				{
					Status: string(database.EventStatusNew),
					Type:   string(database.EventTypeUpdateAirflow),
					Owner:  teamOneID,
				},
				{
					Status: string(database.EventStatusNew),
					Type:   string(database.EventTypeUpdateAirflow),
					Owner:  teamTwoID,
				},
			},
			expected: []gensql.Event{
				{
					Status: string(database.EventStatusNew),
					Type:   string(database.EventTypeUpdateJupyter),
					Owner:  teamOneID,
				},
				{
					Status: string(database.EventStatusNew),
					Type:   string(database.EventTypeUpdateAirflow),
					Owner:  teamTwoID,
				},
			},
		},
	}

	for _, tt := range airflowEventsOmittedTests {
		t.Run(tt.name, func(t *testing.T) {
			userMock := newUserMock()
			teamMock := newTeamMock()
			chartMock := newChartMock()
			helmMock := newHelmMock()
			airflowMock := newAirflowMock()
			handler := EventHandler{
				repo:                       &database.RepoMock{},
				userClient:                 &userMock,
				teamClient:                 &teamMock,
				chartClient:                &chartMock,
				helmClient:                 &helmMock,
				airflowClient:              &airflowMock,
				maintenanceExclusionConfig: maintenanceExclusionConfig,
			}

			got := handler.omitAirflowEventsIfUpgradesPaused(tt.events)
			require.Len(t, got, len(tt.expected))
			diff := cmp.Diff(tt.expected, got)
			assert.Empty(t, diff)
		})
	}
}
