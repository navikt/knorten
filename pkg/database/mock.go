package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/knorten/pkg/database/gensql"
)

type RepoMock struct{}

func (r *RepoMock) EventSetStatus(ctx context.Context, id uuid.UUID, status EventStatus) error {
	return nil
}

func (r *RepoMock) EventSetPendingStatus(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (r *RepoMock) DispatcherEventsGet(ctx context.Context) ([]gensql.DispatcherEventsGetRow, error) {
	return nil, nil
}

func (r *RepoMock) EventLogCreate(ctx context.Context, id uuid.UUID, message string, logType LogType) error {
	return nil
}
