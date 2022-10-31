package helm

import (
	"fmt"
	"log"

	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
)

type HelmClient struct {
	settings *cli.EnvSettings
	log      *logrus.Entry
}

func New(log *logrus.Entry) *HelmClient {
	settings := cli.New()

	return &HelmClient{
		settings: settings,
		log:      log,
	}
}

func getChart(chartName, version, repo string, actionConfig *action.Configuration, settings *cli.EnvSettings) (*chart.Chart, error) {
	destDir := "/tmp"
	pullClient := action.NewPullWithOpts(action.WithConfig(actionConfig))
	pullClient.RepoURL = repo
	pullClient.Settings = settings
	pullClient.DestDir = destDir
	pullClient.Version = version
	_, err := pullClient.Run(chartName)
	if err != nil {
		log.Panicf("failed to pull chart: %v, error: %v", chartName, err)
	}

	return loader.Load(fmt.Sprintf("%v/%v-%v.tgz", destDir, chartName, version))
}

func (h *HelmClient) InstallOrUpgrade()
