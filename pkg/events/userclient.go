package events

import (
	"context"

	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/logger"
)

type userClient interface {
	CreateComputeInstance(ctx context.Context, instance gensql.ComputeInstance, log logger.Logger) bool
	ResizeComputeInstanceDisk(ctx context.Context, instance gensql.ComputeInstance, log logger.Logger) bool
	SyncComputeInstance(ctx context.Context, instance gensql.ComputeInstance, log logger.Logger) bool
	DeleteComputeInstance(ctx context.Context, email string, log logger.Logger) bool
	CreateUserGSM(ctx context.Context, manager gensql.UserGoogleSecretManager, log logger.Logger) bool
	DeleteUserGSM(ctx context.Context, email string, log logger.Logger) bool
}

type userMock struct {
	EventCounts map[database.EventType]int
}

func newUserMock() userMock {
	return userMock{
		EventCounts: map[database.EventType]int{},
	}
}

func (cm userMock) CreateComputeInstance(ctx context.Context, instance gensql.ComputeInstance, log logger.Logger) bool {
	cm.EventCounts[database.EventTypeCreateCompute]++
	return false
}

func (cm userMock) ResizeComputeInstanceDisk(ctx context.Context, instance gensql.ComputeInstance, log logger.Logger) bool {
	cm.EventCounts[database.EventTypeResizeCompute]++
	return false
}

func (cm userMock) SyncComputeInstance(ctx context.Context, instance gensql.ComputeInstance, log logger.Logger) bool {
	cm.EventCounts[database.EventTypeSyncCompute]++
	return false
}

func (cm userMock) DeleteComputeInstance(ctx context.Context, owner string, log logger.Logger) bool {
	cm.EventCounts[database.EventTypeDeleteCompute]++
	return false
}

func (cm userMock) CreateUserGSM(ctx context.Context, manager gensql.UserGoogleSecretManager, log logger.Logger) bool {
	cm.EventCounts[database.EventTypeCreateUserGSM]++
	return false
}

func (cm userMock) DeleteUserGSM(ctx context.Context, owner string, log logger.Logger) bool {
	cm.EventCounts[database.EventTypeDeleteUserGSM]++
	return false
}
