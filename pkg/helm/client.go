package helm

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"

	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/k8s"
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

const (
	helmTimeout = 30 * time.Minute
)

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

func (c *Client) InstallOrUpgrade(ctx context.Context, releaseName, teamID string, app Application) {
	if c.dryRun {
		c.log.Infof("NOOP: Running in dry run mode")
		charty, err := app.Chart(context.Background())
		if err != nil {
			c.log.WithError(err).Errorf("error while generating chart for %v", releaseName)
			return
		}

		out, err := yaml.Marshal(charty.Values)
		if err != nil {
			c.log.WithError(err).Errorf("error while marshaling chart for %v", releaseName)
			return
		}

		err = os.WriteFile(fmt.Sprintf("%v.yaml", releaseName), out, 0o644)
		if err != nil {
			c.log.WithError(err).Errorf("error while writing to file %v.yaml", releaseName)
			return
		}
		return
	}

	namespace := k8s.NameToNamespace(teamID)

	if err := c.repo.TeamSetPendingUpgrade(ctx, teamID, releaseNameToChartType(releaseName), true); err != nil {
		c.log.WithError(err).Errorf("install or upgrading release %v, error setting pending upgrade lock", releaseName)
		return
	}
	defer c.clearPendingUpgrade(ctx, teamID, releaseName)

	settings := cli.New()
	settings.SetNamespace(namespace)
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), "secret", log.Printf); err != nil {
		c.log.WithError(err).Errorf("error while init actionConfig for %v", releaseName)
		return
	}

	listClient := action.NewList(actionConfig)
	listClient.Deployed = true
	results, err := listClient.Run()
	if err != nil {
		c.log.WithError(err).Errorf("error while listing helm releases %v", releaseName)
		return
	}

	exists := false
	for _, rel := range results {
		if rel.Name == releaseName {
			exists = true
		}
	}

	charty, err := app.Chart(context.Background())
	if err != nil {
		c.log.WithError(err).Errorf("error generating chart for %v", releaseName)
		return
	}

	if !exists {
		c.log.Infof("Installing release %v", releaseName)
		installClient := action.NewInstall(actionConfig)
		installClient.Namespace = namespace
		installClient.ReleaseName = releaseName
		installClient.Timeout = helmTimeout

		_, err = installClient.Run(charty, charty.Values)
		if err != nil {
			c.log.WithError(err).Errorf("error while installing release %v", releaseName)
			return
		}
	} else {
		c.log.Infof("Upgrading existing release %v", releaseName)
		upgradeClient := action.NewUpgrade(actionConfig)
		upgradeClient.Namespace = namespace
		upgradeClient.Atomic = true
		upgradeClient.Timeout = helmTimeout

		_, err = upgradeClient.Run(releaseName, charty, charty.Values)
		if err != nil {
			c.log.WithError(err).Errorf("error while upgrading release %v", releaseName)
			return
		}
	}
}

func (c *Client) Uninstall(releaseName, namespace string) {
	if c.dryRun {
		c.log.Infof("NOOP: Running in dry run mode")
		return
	}

	settings := cli.New()
	settings.SetNamespace(namespace)
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), "secret", log.Printf); err != nil {
		c.log.WithError(err).Errorf("error while init actionConfig for %v", releaseName)
		return
	}

	listClient := action.NewList(actionConfig)
	listClient.Deployed = true
	results, err := listClient.Run()
	if err != nil {
		c.log.WithError(err).Errorf("error while listing helm releases %v", releaseName)
		return
	}

	if !releaseExists(results, releaseName) {
		c.log.Infof("release %v does not exist", releaseName)
		return
	}

	uninstallClient := action.NewUninstall(actionConfig)
	_, err = uninstallClient.Run(releaseName)
	if err != nil {
		c.log.WithError(err).Errorf("error while uninstalling release %v", releaseName)
		return
	}
}

func (c *Client) clearPendingUpgrade(ctx context.Context, teamID, releaseName string) {
	if err := c.repo.TeamSetPendingUpgrade(ctx, teamID, releaseNameToChartType(releaseName), false); err != nil {
		c.log.WithError(err).Errorf("install or upgrading release %v, error clearing pending upgrade lock", releaseName)
	}
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

func releaseExists(releases []*release.Release, releaseName string) bool {
	for _, r := range releases {
		if r.Name == releaseName {
			return true
		}
	}

	return false
}

func releaseNameToChartType(releaseName string) string {
	return strings.Split(releaseName, "-")[0]
}
