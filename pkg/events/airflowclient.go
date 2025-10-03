package events

import (
	"context"

	"github.com/navikt/knorten/pkg/database"
)

type airflowClient interface {
	DeleteSchedulerPods(ctx context.Context, teamID string) error
}

type airflowMock struct {
	EventCounts map[database.EventType]int
}

func newAirflowMock() airflowMock {
	return airflowMock{
		EventCounts: map[database.EventType]int{},
	}
}

func (ac airflowMock) DeleteSchedulerPods(ctx context.Context, teamID string) error {
	ac.EventCounts[database.EventTypeDeleteSchedulerPods]++
	return nil
}
