package service

import (
	"context"
	"fmt"

	"github.com/navikt/knorten/pkg/k8s"
	v1 "k8s.io/api/core/v1"
)

type AirflowService interface {
	IsSchedulerDown(ctx context.Context, namespace string) (bool, error)
}

type airflowService struct {
	k8sManager k8s.Manager
}

const airflowSchedulerLabel = "component=scheduler"

func (ac *airflowService) IsSchedulerDown(ctx context.Context, namespace string) (bool, error) {
	statuses, err := ac.k8sManager.GetStatusForPodsWithLabels(ctx, namespace, airflowSchedulerLabel)
	if err != nil {
		return false, fmt.Errorf("is scheduler running: %w", err)
	}

	for _, status := range statuses {
		fmt.Println("status phase", status.Phase)
		if status.Phase == v1.PodRunning {
			return false, nil
		}
	}

	return true, nil
}

func NewAirflowService(k8sManager k8s.Manager) AirflowService {
	return &airflowService{
		k8sManager: k8sManager,
	}
}
