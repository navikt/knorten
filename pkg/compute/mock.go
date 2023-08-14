package compute

import (
	"context"

	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/logger"
)

type ClientMock struct {
	EventCounts map[gensql.EventType]int
}

func NewClientMock() ClientMock {
	return ClientMock{
		EventCounts: map[gensql.EventType]int{},
	}
}

func (cm ClientMock) Create(ctx context.Context, instance gensql.ComputeInstance, log logger.Logger) bool {
	cm.EventCounts[gensql.EventTypeCreateCompute]++
	return false
}

func (cm ClientMock) Delete(ctx context.Context, email string, log logger.Logger) bool {
	cm.EventCounts[gensql.EventTypeDeleteCompute]++
	return false
}
