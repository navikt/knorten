package helm

import (
	"fmt"
	"os"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
)

func FetchChart(repo, chartName, version string) (*chart.Chart, error) {
	settings := cli.New()
	chartRef := fmt.Sprintf("%v/%v", repo, chartName)
	destDir := "/tmp"

	registryClient, err := registry.NewClient(
		registry.ClientOptDebug(settings.Debug),
		registry.ClientOptEnableCache(true),
		registry.ClientOptWriter(os.Stderr),
		registry.ClientOptCredentialsFile(settings.RegistryConfig),
	)
	if err != nil {
		return nil, err
	}

	actionConfig := new(action.Configuration)
	actionConfig.RegistryClient = registryClient
	client := action.NewPullWithOpts(action.WithConfig(actionConfig))
	client.Settings = settings
	client.DestDir = destDir
	client.Version = version

	_, err = client.Run(chartRef)
	if err != nil {
		return nil, err
	}

	return loader.Load(fmt.Sprintf("%v/%v-%v.tgz", destDir, chartName, version))
}
