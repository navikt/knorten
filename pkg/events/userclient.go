package events

import (
	"context"

	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/database/gensql"
)

type userClient interface {
	CreateComputeInstance(ctx context.Context, instance *gensql.ComputeInstance) error
	ResizeComputeInstanceDisk(ctx context.Context, instance *gensql.ComputeInstance) error
	DeleteComputeInstance(ctx context.Context, email string) error
	CreateUserGSM(ctx context.Context, manager *gensql.UserGoogleSecretManager) error
	DeleteUserGSM(ctx context.Context, email string) error
}

type userMock struct {
	EventCounts map[database.EventType]int
}

func newUserMock() userMock {
	return userMock{
		EventCounts: map[database.EventType]int{},
	}
}

func (cm userMock) CreateComputeInstance(ctx context.Context, instance *gensql.ComputeInstance) error {
	cm.EventCounts[database.EventTypeCreateCompute]++
	return nil
}

func (cm userMock) ResizeComputeInstanceDisk(ctx context.Context, instance *gensql.ComputeInstance) error {
	cm.EventCounts[database.EventTypeResizeCompute]++
	return nil
}

func (cm userMock) DeleteComputeInstance(ctx context.Context, owner string) error {
	cm.EventCounts[database.EventTypeDeleteCompute]++
	return nil
}

func (cm userMock) CreateUserGSM(ctx context.Context, manager *gensql.UserGoogleSecretManager) error {
	cm.EventCounts[database.EventTypeCreateUserGSM]++
	return nil
}

func (cm userMock) DeleteUserGSM(ctx context.Context, owner string) error {
	cm.EventCounts[database.EventTypeDeleteUserGSM]++
	return nil
}
