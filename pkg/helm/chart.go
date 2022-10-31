package helm

import (
	"fmt"
	"log"
	"os"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
)

func FetchChart(chartName, version, repo string) (*chart.Chart, error) {
	settings := cli.New()
	settings.SetNamespace("nada")

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), "secret", log.Printf); err != nil {
		log.Printf("%+v", err)
		os.Exit(1)
	}
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
