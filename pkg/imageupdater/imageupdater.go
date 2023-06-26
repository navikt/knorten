package imageupdater

import (
	"context"
	"time"

	"github.com/nais/knorten/pkg/auth"
	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/crypto"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/google"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/sirupsen/logrus"
)

type ImageUpdater struct {
	repo          *database.Repo
	jupyterClient *chart.JupyterhubClient
	airflowClient *chart.AirflowClient
	log           *logrus.Entry
}

type garImage struct {
	Name string `json:"package"`
	Tag  string `json:"tags"`
}

func New(repo *database.Repo, googleClient *google.Google, k8sClient *k8s.Client, azureClient *auth.Azure, cryptClient *crypto.EncrypterDecrypter, jupyterChartVersion, airflowChartVersion string, log *logrus.Entry) *ImageUpdater {
	jupyterClient := chart.NewJupyterhubClient(repo, k8sClient, azureClient, cryptClient, jupyterChartVersion, log.WithField("subsystem", "jupyterClient"))
	airflowClient := chart.NewAirflowClient(repo, googleClient, k8sClient, cryptClient, airflowChartVersion, log.WithField("subsystem", "airflowClient"))
	return &ImageUpdater{
		repo:          repo,
		jupyterClient: &jupyterClient,
		airflowClient: &airflowClient,
		log:           log,
	}
}

func (d *ImageUpdater) Run(frequency time.Duration) {
	ctx := context.Background()

	ticker := time.NewTicker(frequency)
	defer ticker.Stop()
	for {
		d.run(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (d *ImageUpdater) run(ctx context.Context) {
	if err := d.updateJupyterhubImages(ctx); err != nil {
		d.log.WithError(err).Error("updating jupyterhub images")
	}

	// if err := d.updateAirflowBaseImage(ctx); err != nil {
	// 	d.log.WithError(err).Error("updating airflow base image")
	// }
}

func (d *ImageUpdater) triggerSync(ctx context.Context, chartType gensql.ChartType) error {
	teams, err := d.repo.TeamsForAppGet(ctx, chartType)
	if err != nil {
		d.log.WithError(err).Errorf("reading jupyterhub teams from db")
		return err
	}

	for _, t := range teams {
		switch chartType {
		case gensql.ChartTypeAirflow:
			if err := d.airflowClient.Sync(ctx, t); err != nil {
				d.log.WithError(err).Errorf("error syncing airflow for team %v", t)
				return err
			}
		case gensql.ChartTypeJupyterhub:
			if err := d.jupyterClient.Sync(ctx, t); err != nil {
				d.log.WithError(err).Errorf("error syncing jupyterhub for team %v", t)
				return err
			}
		}
	}

	return nil
}
