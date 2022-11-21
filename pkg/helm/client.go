package helm

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"helm.sh/helm/v3/pkg/chart"

	"github.com/nais/knorten/pkg/database"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
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

type Chart struct {
	URL  string
	Name string
}

func New(repo *database.Repo, log *logrus.Entry, dryRun, inCluster bool) (*Client, error) {
	if inCluster {
		if err := initRepositories(); err != nil {
			return nil, err
		}
	}
	return &Client{
		repo:   repo,
		log:    log,
		dryRun: dryRun,
	}, nil
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

func initRepositories() error {
	// TODO: Dette burde være config, de har støtte for å laste denne fra fil
	charts := []Chart{
		{
			URL:  "https://jupyterhub.github.io/helm-chart",
			Name: "jupyterhub",
		},
		{
			URL:  "https://airflow.apache.org",
			Name: "apache-airflow",
		},
	}

	settings := cli.New()
	repoFile := settings.RepositoryConfig

	err := os.MkdirAll(filepath.Dir(repoFile), os.ModePerm)
	if err != nil && !os.IsExist(err) {
		return err
	}

	for _, c := range charts {
		if err := addHelmRepository(c.URL, c.Name, repoFile, settings); err != nil {
			return err
		}
	}
	if err := updateHelmRepositories(repoFile, settings); err != nil {
		return err
	}

	return nil
}
