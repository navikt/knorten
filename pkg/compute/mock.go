package compute

import (
	"context"

	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/logger"
)

type ComputeClientMock struct {
	EventCounts map[gensql.EventType]int
}

func NewComputeClientMock() ComputeClientMock {
	return ComputeClientMock{
		EventCounts: map[gensql.EventType]int{},
	}
}

func (cm ComputeClientMock) Create(ctx context.Context, instance gensql.ComputeInstance, log logger.Logger) bool {
	cm.EventCounts[gensql.EventTypeCreateCompute]++
	return false
}

func (cm ComputeClientMock) Delete(ctx context.Context, email string, log logger.Logger) bool {
	cm.EventCounts[gensql.EventTypeDeleteCompute]++
	return false
}
