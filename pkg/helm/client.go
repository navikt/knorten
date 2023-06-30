package helm

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

type Action string

const (
	timeout                       = 30 * time.Minute
	ActionInstallOrUpgrade Action = "install-or-upgrade"
	ActionUninstall        Action = "uninstall"
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

func New(log *logrus.Entry) (*Client, error) {
	if err := UpdateHelmRepositories(); err != nil {
		return nil, err
	}
	return &Client{
		log: log,
	}, nil
}

func (c *Client) InstallOrUpgrade(releaseName, chartVersion, namespace string, values map[string]any) error {
	settings := cli.New()
	settings.SetNamespace(namespace)
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), "secret", log.Printf); err != nil {
		c.log.WithError(err).Errorf("error while init actionConfig for %v", releaseName)
		return err
	}

	var charty *chart.Chart
	var err error
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

	exists, err := c.releaseExists(actionConfig, releaseName)
	if err != nil {
		return err
	}

	if exists {
		c.log.Infof("Upgrading existing release %v", releaseName)
		upgradeClient := action.NewUpgrade(actionConfig)
		upgradeClient.Namespace = namespace
		upgradeClient.Timeout = timeout

		// upgradeClient.Atomic = true
		// Fra doc: The --wait flag will be set automatically if --atomic is used.
		// Dette hindrer post-upgrade hooken som trigger databasemigrasjonsjobben for airflow og dermed blir alle airflow tjenester låst i wait-for-migrations initcontaineren når
		// vi bumper til ny versjon av airflow hvis denne krever db migrasjoner. Tenker vi løser dette annerledes uansett når vi går over til pubsub så kommenterer det ut for nå.

		_, err = upgradeClient.Run(releaseName, charty, charty.Values)
		if err != nil {
			c.log.WithError(err).Errorf("error while upgrading release %v", releaseName)
			return err
		}
	} else {
		c.log.Infof("Installing new release %v", releaseName)
		installClient := action.NewInstall(actionConfig)
		installClient.Namespace = namespace
		installClient.ReleaseName = releaseName
		installClient.Timeout = timeout

		_, err = installClient.Run(charty, charty.Values)
		if err != nil {
			c.log.WithError(err).Errorf("error while installing new release %v", releaseName)
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

	exists, err := c.releaseExists(actionConfig, releaseName)
	if err != nil {
		return err
	}

	if !exists {
		c.log.Infof("release %v does not exist", releaseName)
		return nil
	}

	uninstallClient := action.NewUninstall(actionConfig)
	_, err = uninstallClient.Run(releaseName)
	if err != nil {
		c.log.WithError(err).Errorf("error while uninstalling release %v", releaseName)
		return err
	}

	return nil
}

func (c *Client) releaseExists(actionConfig *action.Configuration, releaseName string) (bool, error) {
	listClient := action.NewList(actionConfig)
	listClient.Deployed = true
	results, err := listClient.Run()
	if err != nil {
		c.log.WithError(err).Errorf("error while listing helm releases %v", releaseName)
		return false, err
	}

	for _, r := range results {
		if r.Name == releaseName {
			return true, nil
		}
	}

	return false, nil
}

func ReleaseNameToChartType(releaseName string) string {
	return strings.Split(releaseName, "-")[0]
}
