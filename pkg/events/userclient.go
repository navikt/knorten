package events

import (
	"context"

	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/database/gensql"
)

type userClient interface {
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

func (cm userMock) CreateUserGSM(ctx context.Context, manager *gensql.UserGoogleSecretManager) error {
	cm.EventCounts[database.EventTypeCreateUserGSM]++
	return nil
}

func (cm userMock) DeleteUserGSM(ctx context.Context, owner string) error {
	cm.EventCounts[database.EventTypeDeleteUserGSM]++
	return nil
}
