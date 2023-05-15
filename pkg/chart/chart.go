package chart

import (
	"github.com/nais/knorten/pkg/auth"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/crypto"
	"github.com/nais/knorten/pkg/google"
	"github.com/nais/knorten/pkg/helm"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/sirupsen/logrus"
)

type Client struct {
	Airflow    AirflowClient
	Jupyterhub JupyterhubClient
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
