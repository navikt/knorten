package team

import (
	"context"
	"fmt"

	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/k8s"
)

type AirflowClient struct {
	repo    *database.Repo
	manager k8s.Manager
}

func NewAirflowClient(
	repo *database.Repo,
	mngr k8s.Manager,
) (*AirflowClient, error) {
	return &AirflowClient{
		repo:    repo,
		manager: mngr,
	}, nil
}

const airflowSchedulerLabel = "component=scheduler"

func (ac AirflowClient) DeleteSchedulerPods(ctx context.Context, namespace string) error {
	err := ac.manager.DeletePodsWithLabels(ctx, namespace, airflowSchedulerLabel)
	if err != nil {
		return fmt.Errorf("delete scheduler pods: %w", err)
	}
	return nil
}
