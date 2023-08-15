package events

import (
	"context"

	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/logger"
)

type computeClient interface {
	Create(ctx context.Context, instance gensql.ComputeInstance, log logger.Logger) bool
	Delete(ctx context.Context, email string, log logger.Logger) bool
}

type computeMock struct {
	EventCounts map[database.EventType]int
}

func newComputeMock() computeMock {
	return computeMock{
		EventCounts: map[database.EventType]int{},
	}
}

func (cm computeMock) Create(ctx context.Context, instance gensql.ComputeInstance, log logger.Logger) bool {
	cm.EventCounts[database.EventTypeCreateCompute]++
	return false
}

func (cm computeMock) Delete(ctx context.Context, owner string, log logger.Logger) bool {
	cm.EventCounts[database.EventTypeDeleteCompute]++
	return false
}
