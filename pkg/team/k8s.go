package team

import (
	"context"
	"fmt"

	"github.com/navikt/knorten/pkg/k8s"
	v1 "k8s.io/api/core/v1"
)

type AirflowClient struct {
	manager k8s.Manager
}

func NewAirflowClient(mngr k8s.Manager) *AirflowClient {
	return &AirflowClient{
		manager: mngr,
	}
}

const airflowSchedulerLabel = "component=scheduler"

func (ac AirflowClient) DeleteSchedulerPods(ctx context.Context, namespace string) error {
	err := ac.manager.DeletePodsWithLabels(ctx, namespace, airflowSchedulerLabel)
	if err != nil {
		return fmt.Errorf("delete scheduler pods: %w", err)
	}
	return nil
}

func (ac *AirflowClient) IsSchedulerDown(ctx context.Context, namespace string) (bool, error) {
	statuses, err := ac.manager.GetStatusForPodsWithLabels(ctx, namespace, airflowSchedulerLabel)
	if err != nil {
		return false, fmt.Errorf("is scheduler running: %w", err)
	}

	if len(statuses) == 0 {
		// No scheduler pods found for team
		return true, nil
	}

	for _, status := range statuses {
		if status.Phase == v1.PodRunning {
			return false, nil
		}
	}

	return true, nil
}
