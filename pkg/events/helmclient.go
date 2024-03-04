package events

import (
	"context"

	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/helm"
	"github.com/navikt/knorten/pkg/logger"
)

type helmClient interface {
	InstallOrUpgrade(ctx context.Context, helmData helm.EventData, log logger.Logger) error
	Rollback(ctx context.Context, helmData helm.EventData, log logger.Logger) (bool, error)
	Uninstall(ctx context.Context, helmData helm.EventData, log logger.Logger) bool
}

type helmMock struct {
	EventCounts map[database.EventType]int
}

func newHelmMock() helmMock {
	return helmMock{
		EventCounts: map[database.EventType]int{},
	}
}

func (hm helmMock) InstallOrUpgrade(ctx context.Context, helmEvent helm.EventData, logger logger.Logger) error {
	hm.EventCounts[database.EventTypeHelmRolloutJupyter]++
	hm.EventCounts[database.EventTypeHelmRolloutAirflow]++
	return nil
}

func (hm helmMock) Rollback(ctx context.Context, helmEvent helm.EventData, logger logger.Logger) (bool, error) {
	hm.EventCounts[database.EventTypeHelmRollbackJupyter]++
	hm.EventCounts[database.EventTypeHelmRollbackAirflow]++
	return false, nil
}

func (hm helmMock) Uninstall(ctx context.Context, helmEvent helm.EventData, logger logger.Logger) bool {
	hm.EventCounts[database.EventTypeHelmUninstallJupyter]++
	hm.EventCounts[database.EventTypeHelmUninstallAirflow]++
	return false
}
