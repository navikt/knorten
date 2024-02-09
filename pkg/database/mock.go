package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/navikt/knorten/pkg/database/gensql"
)

type RepoMock struct{}

func (r *RepoMock) EventSetStatus(ctx context.Context, id uuid.UUID, status EventStatus) error {
	return nil
}

func (r *RepoMock) EventSetPendingStatus(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (r *RepoMock) EventsReset(ctx context.Context) error {
	return nil
}

func (r *RepoMock) DispatcherEventsGet(ctx context.Context) ([]gensql.Event, error) {
	return nil, nil
}

func (r *RepoMock) DispatchableEventsGet(ctx context.Context) ([]gensql.Event, error) {
	return nil, nil
}

func (r *RepoMock) EventLogCreate(ctx context.Context, id uuid.UUID, message string, logType LogType) error {
	return nil
}

func (r *RepoMock) RegisterHelmInstallOrUpgradeEvent(ctx context.Context, teamID string, helmEvent any) error {
	return nil
}

func (r *RepoMock) RegisterHelmRollbackEvent(ctx context.Context, helmEvent any) error {
	return nil
}

func (r *RepoMock) RegisterHelmUninstallEvent(ctx context.Context, helmEvent any) bool {
	return false
}
