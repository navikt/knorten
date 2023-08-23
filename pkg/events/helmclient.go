package events

import (
	"context"

	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/logger"
)

type helmClient interface {
	InstallOrUpgrade(ctx context.Context, helmData database.HelmEvent, log logger.Logger) error
	Rollback(ctx context.Context, helmData database.HelmEvent, log logger.Logger) (bool, error)
	Uninstall(ctx context.Context, helmData database.HelmEvent, log logger.Logger) bool
	HelmTimeoutWatcher(ctx context.Context, helmData database.HelmEvent, log logger.Logger)
}

type helmMock struct {
	EventCounts map[database.EventType]int
}

func newHelmMock() helmMock {
	return helmMock{
		EventCounts: map[database.EventType]int{},
	}
}

func (hm helmMock) InstallOrUpgrade(ctx context.Context, helmEvent database.HelmEvent, logger logger.Logger) error {
	hm.EventCounts[database.EventTypeHelmInstallOrUpgrade]++
	return nil
}

func (hm helmMock) Rollback(ctx context.Context, helmEvent database.HelmEvent, logger logger.Logger) (bool, error) {
	hm.EventCounts[database.EventTypeHelmRollback]++
	return false, nil
}

func (hm helmMock) Uninstall(ctx context.Context, helmEvent database.HelmEvent, logger logger.Logger) bool {
	hm.EventCounts[database.EventTypeHelmUninstall]++
	return false
}

func (hm helmMock) HelmTimeoutWatcher(ctx context.Context, helmData database.HelmEvent, log logger.Logger) {
}
