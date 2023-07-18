package chart

import (
	"context"

	"github.com/nais/knorten/pkg/auth"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/crypto"
	"github.com/nais/knorten/pkg/google"
	"github.com/nais/knorten/pkg/helm"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/sirupsen/logrus"
)

type Client struct {
	airflow    airflowClient
	jupyter    jupyterClient
	Airflow    AirflowClient
	Jupyterhub JupyterhubClient
	log        *logrus.Entry
}

func New(repo *database.Repo, googleClient *google.Google, k8sClient *k8s.Client, azureClient *auth.Azure, cryptClient *crypto.EncrypterDecrypter, airflowChartVersion, jupyterChartVersion string, log *logrus.Entry) (*Client, error) {
	if err := helm.UpdateHelmRepositories(); err != nil {
		return nil, err
	}

	return &Client{
		Airflow:    NewAirflowClient(repo, googleClient, k8sClient, cryptClient, airflowChartVersion, log),
		Jupyterhub: NewJupyterhubClient(repo, k8sClient, azureClient, cryptClient, jupyterChartVersion, log),
	}, nil
}

func NewClient(repo *database.Repo, dryRun, inCluster bool, gcpProject, airflowChartVersion, jupyterChartVersion string, log *logrus.Entry) (*Client, error) {
	k8sClient, err := k8s.CreateClientset(inCluster)
	if err != nil {
		return nil, err
	}

	return &Client{
		airflow: newAirflowClient(repo, k8sClient, dryRun, airflowChartVersion, gcpProject, log.WithField("chart", "airflow")),
		jupyter: newJupyterClient(repo, k8sClient, dryRun, jupyterChartVersion, log.WithField("chart", "jupyter")),
		log:     log,
	}, nil
}

func (c Client) DeleteJupyter(ctx context.Context, teamID string) bool {
	if err := c.jupyter.Delete(ctx, teamID); err != nil {
		c.log.WithError(err).Error("failed deleting jupyter")
		return true
	}

	return false
}

func (c Client) DeleteAirflow(ctx context.Context, teamID string) bool {
	if err := c.airflow.Delete(ctx, teamID); err != nil {
		c.log.WithError(err).Error("failed deleting airflow")
		return true
	}

	return false
}
