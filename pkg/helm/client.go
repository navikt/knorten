package helm

import (
	"context"
	"log"

	"github.com/nais/knorten/pkg/database"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

type Application interface {
	Chart(ctx context.Context) (*chart.Chart, error)
}

type Client struct {
	repo   *database.Repo
	log    *logrus.Entry
	dryRun bool
}

func New(repo *database.Repo, log *logrus.Entry, dryRun bool) *Client {
	return &Client{
		repo:   repo,
		log:    log,
		dryRun: dryRun,
	}
}

func (h *Client) InstallOrUpgrade(releaseName, namespace string, app Application) error {
	if h.dryRun {
		h.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	hChart, err := app.Chart(context.Background())
	if err != nil {
		h.log.WithError(err).Errorf("install or upgrading release %v", releaseName)
	}

	settings := cli.New()
	settings.SetNamespace(namespace)
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), "secret", log.Printf); err != nil {
		log.Printf("%+v", err)
		h.log.WithError(err).Errorf("install or upgrading release %v", releaseName)
		return err
	}

	listClient := action.NewList(actionConfig)
	listClient.Deployed = true
	results, err := listClient.Run()
	if err != nil {
		h.log.WithError(err).Errorf("install or upgrading release %v", releaseName)
		return err
	}

	exists := false
	for _, rel := range results {
		if rel.Name == releaseName {
			exists = true
		}
	}

	if !exists {
		h.log.Infof("Installing release %v", releaseName)
		installClient := action.NewInstall(actionConfig)
		installClient.Namespace = namespace
		installClient.ReleaseName = releaseName

		_, err = installClient.Run(hChart, hChart.Values)
		if err != nil {
			h.log.WithError(err).Errorf("install or upgrading release %v", releaseName)
			return err
		}
	} else {
		h.log.Infof("Upgrading existing release %v", releaseName)
		upgradeClient := action.NewUpgrade(actionConfig)
		upgradeClient.Namespace = namespace

		_, err = upgradeClient.Run(releaseName, hChart, hChart.Values)
		if err != nil {
			h.log.WithError(err).Errorf("install or upgrading release %v", releaseName)
			return err
		}
	}

	return nil
}
