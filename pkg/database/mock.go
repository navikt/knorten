package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/knorten/pkg/database/gensql"
)

type RepoMock struct{}

func (r *RepoMock) EventSetStatus(ctx context.Context, id uuid.UUID, status gensql.EventStatus) error {
	return nil
}

func (r *RepoMock) EventSetPendingStatus(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (r *RepoMock) DispatcherEventsGet(ctx context.Context) ([]gensql.Event, error) {
	return nil, nil
}

func (r *RepoMock) DispatchableEventsGet(ctx context.Context) ([]gensql.Event, error) {
	return nil, nil
}

func (r *RepoMock) EventLogCreate(ctx context.Context, id uuid.UUID, message string, logType gensql.LogType) error {
	return nil
}
