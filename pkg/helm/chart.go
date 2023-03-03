package helm

import (
	"fmt"
	"io/fs"
	"os"

	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/repo"
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

func addHelmRepository(url, chartName, repoFile string, settings *cli.EnvSettings) error {
	bytes, err := os.ReadFile(repoFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var f repo.File
	if err := yaml.Unmarshal(bytes, &f); err != nil {
		return err
	}

	c := repo.Entry{
		Name: chartName,
		URL:  url,
	}

	r, err := repo.NewChartRepository(&c, getter.All(settings))
	if err != nil {
		return err
	}

	if _, err := r.DownloadIndexFile(); err != nil {
		return fmt.Errorf("looks like %q is not a valid chart repository or cannot be reached: %v", url, err)
	}

	f.Update(&c)

	if err := f.WriteFile(repoFile, fs.ModeAppend); err != nil {
		return err
	}

	return nil
}

func UpdateHelmRepositories() error {
	settings := cli.New()
	repoFile := settings.RepositoryConfig

	f, err := repo.LoadFile(repoFile)
	if err != nil {
		return err
	}

	var repos []*repo.ChartRepository
	for _, cfg := range f.Repositories {
		r, err := repo.NewChartRepository(cfg, getter.All(settings))
		if err != nil {
			return err
		}
		repos = append(repos, r)
	}

	for _, re := range repos {
		if _, err := re.DownloadIndexFile(); err != nil {
			return err
		}
	}

	return nil
}
