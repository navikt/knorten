package events

import (
	"context"

	"github.com/navikt/knorten/pkg/database"
)

type airflowClient interface {
	DeleteSchedulerPods(ctx context.Context, namespace string) error
	IsSchedulerDown(ctx context.Context, namespace string) (bool, error)
}

type airflowMock struct {
	EventCounts map[database.EventType]int
}

func newAirflowMock() airflowMock {
	return airflowMock{
		EventCounts: map[database.EventType]int{},
	}
}

func (ac airflowMock) DeleteSchedulerPods(ctx context.Context, namespace string) error {
	ac.EventCounts[database.EventTypeDeleteSchedulerPods]++
	return nil
}

func (ac airflowMock) IsSchedulerDown(ctx context.Context, namespace string) (bool, error) {
	return false, nil
}
