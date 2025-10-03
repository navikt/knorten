package team

import (
	"context"
	"fmt"

	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/k8s"
)

type AirflowClient struct {
	repo       *database.Repo
	manager    k8s.Manager
	gcpProject string
	gcpRegion  string
	dryRun     bool
}

func NewAirflowClient(
	repo *database.Repo,
	mngr k8s.Manager,
	gcpProject, gcpRegion string,
	dryRun bool,
) (*AirflowClient, error) {
	return &AirflowClient{
		repo:       repo,
		manager:    mngr,
		gcpProject: gcpProject,
		gcpRegion:  gcpRegion,
		dryRun:     dryRun,
	}, nil
}

const airflowSchedulerLabel = "component=scheduler"

func (ac AirflowClient) DeleteSchedulerPods(ctx context.Context, teamID string) error {
	err := ac.manager.DeletePodsWithLables(ctx, teamID, airflowSchedulerLabel)
	if err != nil {
		return fmt.Errorf("delete scheduler pods: %w", err)
	}
	return nil
}
