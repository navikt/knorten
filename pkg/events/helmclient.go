package events

import (
	"context"

	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/helm"
)

type helmClient interface {
	InstallOrUpgrade(ctx context.Context, helmData *helm.EventData) error
	Rollback(ctx context.Context, helmData *helm.EventData) error
	Uninstall(ctx context.Context, helmData *helm.EventData) error
}

type helmMock struct {
	EventCounts map[database.EventType]int
}

func newHelmMock() helmMock {
	return helmMock{
		EventCounts: map[database.EventType]int{},
	}
}

func (hm helmMock) InstallOrUpgrade(ctx context.Context, helmEvent *helm.EventData) error {
	hm.EventCounts[database.EventTypeHelmRolloutAirflow]++
	return nil
}

func (hm helmMock) Rollback(ctx context.Context, helmEvent *helm.EventData) error {
	hm.EventCounts[database.EventTypeHelmRollbackAirflow]++
	return nil
}

func (hm helmMock) Uninstall(ctx context.Context, helmEvent *helm.EventData) error {
	hm.EventCounts[database.EventTypeHelmUninstallAirflow]++
	return nil
}
