package team

import (
	"context"
	"fmt"

	"github.com/navikt/knorten/pkg/k8s"
)

type AirflowClient struct {
	manager k8s.Manager
}

func NewAirflowClient(
	mngr k8s.Manager,
) (*AirflowClient, error) {
	return &AirflowClient{
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
