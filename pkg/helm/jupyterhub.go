package helm

import (
	"context"
	"strings"

	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"helm.sh/helm/v3/pkg/chart"
)

type Jupyterhub struct {
	team         string
	templatePath string
	repo         *database.Repo
}

func NewJupyterhub(team, tmplPath string, repo *database.Repo) *Jupyterhub {
	return &Jupyterhub{
		team:         team,
		templatePath: tmplPath,
		repo:         repo,
	}
}

func (j *Jupyterhub) Chart(ctx context.Context) (*chart.Chart, error) {
	chart, err := FetchChart("jupyterhub", "0.11.1", "https://jupyterhub.github.io/helm-chart")
	if err != nil {
		return nil, err
	}

	err = j.mergeValues(ctx, chart.Values)
	if err != nil {
		return nil, err
	}

	return chart, nil
}

func setChartValue(keys []string, value string, chart map[string]any) {
	key := keys[0]
	if len(keys) > 1 {
		setChartValue(keys[1:], value, chart[key].(map[string]any))
		return
	}

	chart[key] = value
}

func (j *Jupyterhub) mergeValues(ctx context.Context, chart map[string]interface{}) error {
	values, err := j.globalValues(ctx)
	if err != nil {
		return err
	}

	values, err = j.teamValues(ctx, values)
	if err != nil {
		return err
	}

	// chartValues + values = success
	for key, value := range values {
		keys := strings.Split(key, ".")
		setChartValue(keys, value, chart)
	}

	return nil
}

func (j *Jupyterhub) globalValues(ctx context.Context) (map[string]string, error) {
	dbValues, err := j.repo.GlobalValuesGet(ctx, gensql.ChartTypeJupyterhub)
	if err != nil {
		return map[string]string{}, err
	}

	values := map[string]string{}
	for _, v := range dbValues {
		values[v.Key] = v.Value
	}

	return values, nil
}

func (j *Jupyterhub) teamValues(ctx context.Context, values map[string]string) (map[string]string, error) {
	dbValues, err := j.repo.TeamValuesGet(ctx, gensql.ChartTypeJupyterhub, j.team)
	if err != nil {
		return map[string]string{}, err
	}

	for _, v := range dbValues {
		values[v.Key] = v.Value
	}

	return values, nil
}
