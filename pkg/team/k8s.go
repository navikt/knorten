package team

import (
	"context"
	"fmt"

	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/k8s"
	v1 "k8s.io/api/core/v1"
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
	err := ac.manager.DeletePodsWithLables(ctx, namespace, airflowSchedulerLabel)
	if err != nil {
		return fmt.Errorf("delete scheduler pods: %w", err)
	}
	return nil
}

func (ac AirflowClient) IsSchedulerDown(ctx context.Context, namespace string) (bool, error) {
	statuses, err := ac.manager.GetStatusForPodsWithLabels(ctx, namespace, airflowSchedulerLabel)
	if err != nil {
		return false, fmt.Errorf("is scheduler running: %w", err)
	}

	for _, status := range statuses {
		if status.Phase == v1.PodRunning {
			return false, nil
		}
	}
	return true, nil
}
