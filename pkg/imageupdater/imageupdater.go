package imageupdater

import (
	"context"
	"time"

	"github.com/nais/knorten/pkg/auth"
	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/crypto"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/sirupsen/logrus"
)

type ImageUpdater struct {
	repo          *database.Repo
	jupyterClient *chart.JupyterhubClient
	log           *logrus.Entry
}

type garImage struct {
	Name string `json:"package"`
	Tag  string `json:"tags"`
}

func New(repo *database.Repo, k8sClient *k8s.Client, azureClient *auth.Azure, cryptClient *crypto.EncrypterDecrypter, jupyterChartVersion string, log *logrus.Entry) *ImageUpdater {
	jupyterClient := chart.NewJupyterhubClient(repo, k8sClient, azureClient, cryptClient, jupyterChartVersion, log.WithField("subsystem", "jupyterClient"))
	return &ImageUpdater{
		repo:          repo,
		jupyterClient: &jupyterClient,
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

	if err := d.updateAirflowBaseImage(ctx); err != nil {
		d.log.WithError(err).Error("updating airflow base image")
	}
}
