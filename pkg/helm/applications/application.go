package helm

import (
	"context"

	"helm.sh/helm/v3/pkg/chart"

	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/helm"
)

type Application struct {
	chartName    string
	chartRepo    string
	chartType    gensql.ChartType
	chartVersion string
	team         string
	repo         *database.Repo
}

// TODO: Vi bør ta inn chart-settings som config

func NewAirflow(team string, repo *database.Repo) *Application {
	return &Application{
		chartName:    "airflow",
		chartRepo:    "apache-airflow",
		chartType:    gensql.ChartTypeJupyterhub,
		chartVersion: "1.7.0",
		team:         team,
		repo:         repo,
	}
}

func NewJupyterhub(team string, repo *database.Repo) *Application {
	return &Application{
		chartName:    "jupyterhub",
		chartRepo:    "jupyterhub",
		chartType:    gensql.ChartTypeJupyterhub,
		chartVersion: "2.0.0",
		team:         team,
		repo:         repo,
	}
}

func NewNamespace(team string, repo *database.Repo) *Application {
	return &Application{
		chartName:    "knada-namespace-setup",
		chartRepo:    "oci://europe-west1-docker.pkg.dev/knada-gcp/helm",
		chartType:    gensql.ChartTypeNamespace,
		chartVersion: "0.1.10",
		team:         team,
		repo:         repo,
	}
}

func (a *Application) Chart(ctx context.Context) (*chart.Chart, error) {
	chart, err := helm.FetchChart(a.chartRepo, a.chartName, a.chartVersion)
	if err != nil {
		return nil, err
	}

	err = a.mergeValues(ctx, chart.Values)
	if err != nil {
		return nil, err
	}

	return chart, nil
}

func (a *Application) mergeValues(ctx context.Context, defaultValues map[string]any) error {
	values, err := a.globalValues(ctx)
	if err != nil {
		return err
	}

	values, err = a.enrichWithTeamValues(ctx, values)
	if err != nil {
		return err
	}

	for key, value := range values {
		keyPath := helm.KeySplitHandleEscape(key)
		helm.SetChartValue(keyPath, value, defaultValues)
	}

	return nil
}

func (a *Application) globalValues(ctx context.Context) (map[string]any, error) {
	dbValues, err := a.repo.GlobalValuesGet(ctx, a.chartType)
	if err != nil {
		return map[string]any{}, err
	}

	values := map[string]any{}
	for _, v := range dbValues {
		values[v.Key], err = helm.ParseValue(v.Value)
		if err != nil {
			return nil, err
		}
	}

	return values, nil
}

func (a *Application) enrichWithTeamValues(ctx context.Context, values map[string]any) (map[string]any, error) {
	dbValues, err := a.repo.TeamValuesGet(ctx, a.chartType, a.team)
	if err != nil {
		return map[string]any{}, err
	}

	for _, v := range dbValues {
		values[v.Key], err = helm.ParseValue(v.Value)
		if err != nil {
			return nil, err
		}
	}

	return values, nil
}
