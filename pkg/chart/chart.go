package chart

import (
	"context"

	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/helm"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/nais/knorten/pkg/logger"
	"k8s.io/client-go/kubernetes"
)

type Client struct {
	repo                *database.Repo
	k8sClient           *kubernetes.Clientset
	dryRun              bool
	chartVersionAirflow string
	chartVersionJupyter string
	gcpProject          string
	gcpRegion           string
}

func NewClient(repo *database.Repo, dryRun, inCluster bool, airflowChartVersion, jupyterChartVersion, gcpProject, gcpRegion string) (*Client, error) {
	if err := helm.UpdateHelmRepositories(); err != nil {
		return nil, err
	}

	k8sClient, err := k8s.CreateClientset(inCluster)
	if err != nil {
		return nil, err
	}

	return &Client{
		repo:                repo,
		k8sClient:           k8sClient,
		dryRun:              dryRun,
		chartVersionJupyter: jupyterChartVersion,
		chartVersionAirflow: airflowChartVersion,
		gcpProject:          gcpProject,
		gcpRegion:           gcpRegion,
	}, nil
}

func (c Client) SyncJupyter(ctx context.Context, values JupyterConfigurableValues, log logger.Logger) bool {
	if err := c.syncJupyter(ctx, values); err != nil {
		log.WithError(err).WithField("team", values.Slug).Error("failed creating jupyter")
		return true
	}

	return false
}

func (c Client) DeleteJupyter(ctx context.Context, teamID string, log logger.Logger) bool {
	if err := c.deleteJupyter(ctx, teamID); err != nil {
		log.WithError(err).WithField("team", teamID).Error("failed deleting jupyter")
		return true
	}

	return false
}

func (c Client) SyncAirflow(ctx context.Context, values AirflowConfigurableValues, log logger.Logger) bool {
	if err := c.syncAirflow(ctx, values); err != nil {
		log.WithError(err).WithField("team", values.Slug).Error("failed creating jupyter")
		return true
	}

	return false
}

func (c Client) DeleteAirflow(ctx context.Context, teamID string, log logger.Logger) bool {
	if err := c.deleteAirflow(ctx, teamID); err != nil {
		log.WithError(err).WithField("team", teamID).Error("failed deleting airflow")
		return true
	}

	return false
}
