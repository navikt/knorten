package chart

import (
	"context"

	"github.com/nais/knorten/pkg/api/auth"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/nais/knorten/pkg/logger"
	"k8s.io/client-go/kubernetes"
)

type Client struct {
	repo                *database.Repo
	k8sClient           *kubernetes.Clientset
	azureClient         *auth.Azure
	dryRun              bool
	chartVersionAirflow string
	chartVersionJupyter string
	gcpProject          string
	gcpRegion           string
}

func NewClient(repo *database.Repo, azureClient *auth.Azure, dryRun, inCluster bool, airflowChartVersion, jupyterChartVersion, gcpProject, gcpRegion string) (*Client, error) {
	k8sClient, err := k8s.CreateClientset(dryRun, inCluster)
	if err != nil {
		return nil, err
	}

	return &Client{
		repo:                repo,
		azureClient:         azureClient,
		k8sClient:           k8sClient,
		dryRun:              dryRun,
		chartVersionJupyter: jupyterChartVersion,
		chartVersionAirflow: airflowChartVersion,
		gcpProject:          gcpProject,
		gcpRegion:           gcpRegion,
	}, nil
}

func (c Client) SyncJupyter(ctx context.Context, values JupyterConfigurableValues, log logger.Logger) bool {
	log.Info("Syncing Jupyter")

	if err := c.syncJupyter(ctx, values, log); err != nil {
		log.Info("Failed syncing Jupyter")
		return true
	}

	log.Info("Successfully synced Jupyter")
	return false
}

func (c Client) DeleteJupyter(ctx context.Context, teamID string, log logger.Logger) bool {
	log.Info("Deleting Jupyter")

	if err := c.deleteJupyter(ctx, teamID, log); err != nil {
		log.Info("Failed deleting Jupyter")
		return true
	}

	log.Info("Successfully deleted Jupyter")
	return false
}

func (c Client) SyncAirflow(ctx context.Context, values AirflowConfigurableValues, log logger.Logger) bool {
	log.Info("Syncing Airflow")

	if err := c.syncAirflow(ctx, values, log); err != nil {
		log.Info("Failed syncing Airflow")
		return true
	}

	log.Info("Successfully synced Airflow")
	return false
}

func (c Client) DeleteAirflow(ctx context.Context, teamID string, log logger.Logger) bool {
	log.Info("Deleting Airflow")

	if err := c.deleteAirflow(ctx, teamID, log); err != nil {
		log.Info("Failed deleting Airflow")
		return true
	}

	log.Info("Successfully deleted Airflow")
	return false
}
