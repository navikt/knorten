package chart

import (
	"context"

	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
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

func (c Client) UpdateJupyter(ctx context.Context, values JupyterConfigurableValues, log logger.Logger) bool {
	log.WithField("team", values.Slug).WithField("chart", "jupyter").Info("Updating Jupyter")
	apps, err := c.repo.AppsForTeamGet(ctx, values.Slug)
	if err != nil {
		log.WithField("team", values.Slug).WithField("chart", "jupyter").WithError(err).Error("failed getting apps for team")
		return true
	}

	retry := false
	for _, app := range apps {
		if app == string(gensql.ChartTypeJupyterhub) {
			retry = c.SyncJupyter(ctx, values, log)
		}
	}

	log.WithField("team", values.Slug).WithField("chart", "jupyter").Info("Successfully updated Jupyter")
	return retry
}

func (c Client) SyncJupyter(ctx context.Context, values JupyterConfigurableValues, log logger.Logger) bool {
	log = log.WithField("team", values.Slug).WithField("chart", "jupyter")
	log.Info("Syncing Jupyter")

	if err := c.syncJupyter(ctx, values); err != nil {
		log.WithError(err).WithField("team", values.Slug).Error("failed syncing Jupyter")
		return true
	}

	log.Info("Successfully synced Jupyter")
	return false
}

func (c Client) DeleteJupyter(ctx context.Context, teamID string, log logger.Logger) bool {
	log = log.WithField("team", teamID).WithField("chart", "jupyter")
	log.Info("Deleting Jupyter")

	if err := c.deleteJupyter(ctx, teamID); err != nil {
		log.WithError(err).WithField("team", teamID).Error("failed deleting Jupyter")
		return true
	}

	log.Info("Successfully deleted Jupyter")
	return false
}

func (c Client) UpdateAirflow(ctx context.Context, values AirflowConfigurableValues, log logger.Logger) bool {
	log.WithField("team", values.Slug).WithField("chart", "jupyter").Info("Updating Airflow")
	apps, err := c.repo.AppsForTeamGet(ctx, values.Slug)
	if err != nil {
		log.WithField("team", values.Slug).WithField("chart", "airflow").WithError(err).Error("failed getting apps for team")
		return true
	}

	retry := false
	for _, app := range apps {
		if app == string(gensql.ChartTypeAirflow) {
			retry = c.SyncAirflow(ctx, values, log)
		}
	}

	log.WithField("team", values.Slug).WithField("chart", "airflow").Info("Successfully updated Airflow")
	return retry
}

func (c Client) SyncAirflow(ctx context.Context, values AirflowConfigurableValues, log logger.Logger) bool {
	log = log.WithField("team", values.Slug).WithField("chart", "airflow")
	log.Info("Syncing Airflow")

	if err := c.syncAirflow(ctx, values); err != nil {
		log.WithError(err).WithField("team", values.Slug).Error("failed syncing Airflow")
		return true
	}

	log.Info("Successfully synced Airflow")
	return false
}

func (c Client) DeleteAirflow(ctx context.Context, teamID string, log logger.Logger) bool {
	log = log.WithField("team", teamID).WithField("chart", "airflow")
	log.Info("Deleting Airflow")

	if err := c.deleteAirflow(ctx, teamID); err != nil {
		log.WithError(err).WithField("team", teamID).Error("failed deleting Airflow")
		return true
	}

	log.Info("Successfully deleted Airflow")
	return false
}
