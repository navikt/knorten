package helm

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"

	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

type Application interface {
	Chart(ctx context.Context) (*chart.Chart, error)
}

type Client struct {
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

func New(log *logrus.Entry) (*Client, error) {
	if err := UpdateHelmRepositories(); err != nil {
		return nil, err
	}
	return &Client{
		log: log,
	}, nil
}

func (c *Client) InstallOrUpgrade(ctx context.Context, releaseName, chartVersion, namespace string, values map[string]any) error {
	settings := cli.New()
	settings.SetNamespace(namespace)
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), "secret", log.Printf); err != nil {
		c.log.WithError(err).Errorf("error while init actionConfig for %v", releaseName)
		return err
	}

	listClient := action.NewList(actionConfig)
	results, err := listClient.Run()
	if err != nil {
		c.log.WithError(err).Errorf("error while listing helm releases %v", releaseName)
		return err
	}

	exists := false
	for _, rel := range results {
		if rel.Name == releaseName {
			exists = true
		}
	}

	var charty *chart.Chart
	switch ReleaseNameToChartType(releaseName) {
	case string(gensql.ChartTypeJupyterhub):
		charty, err = FetchChart("jupyterhub", "jupyterhub", chartVersion)
	case string(gensql.ChartTypeAirflow):
		charty, err = FetchChart("apache-airflow", "airflow", chartVersion)
	default:
		return fmt.Errorf("chart type for release %v is not supported", releaseName)
	}
	if err != nil {
		c.log.WithError(err).Errorf("error fetching chart for %v", releaseName)
		return err
	}

	charty.Values = values

	if !exists {
		c.log.Infof("Installing release %v", releaseName)
		installClient := action.NewInstall(actionConfig)
		installClient.Namespace = namespace
		installClient.ReleaseName = releaseName
		installClient.Timeout = helmTimeout

		_, err = installClient.Run(charty, charty.Values)
		if err != nil {
			c.log.WithError(err).Errorf("error while installing release %v", releaseName)
			return err
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
			return err
		}
	}

	return nil
}

func (c *Client) Uninstall(releaseName, namespace string) error {
	settings := cli.New()
	settings.SetNamespace(namespace)
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), "secret", log.Printf); err != nil {
		c.log.WithError(err).Errorf("error while init actionConfig for %v", releaseName)
		return err
	}

	listClient := action.NewList(actionConfig)
	listClient.Deployed = true
	results, err := listClient.Run()
	if err != nil {
		c.log.WithError(err).Errorf("error while listing helm releases %v", releaseName)
		return err
	}

	if !releaseExists(results, releaseName) {
		c.log.Infof("release %v does not exist", releaseName)
		return err
	}

	uninstallClient := action.NewUninstall(actionConfig)
	_, err = uninstallClient.Run(releaseName)
	if err != nil {
		c.log.WithError(err).Errorf("error while uninstalling release %v", releaseName)
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

func ReleaseNameToChartType(releaseName string) string {
	return strings.Split(releaseName, "-")[0]
}
