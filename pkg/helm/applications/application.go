package helm

import (
	"context"
	"strconv"
	"strings"

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
		chartType:    gensql.ChartTypeAirflow,
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
		chartVersion: "0.11.1",
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

	err = a.enrichWithTeamValues(ctx, values)
	if err != nil {
		return err
	}

	mergeMaps(defaultValues, values)
	return nil
}

func (a *Application) globalValues(ctx context.Context) (map[string]any, error) {
	dbValues, err := a.repo.GlobalValuesGet(ctx, a.chartType)
	if err != nil {
		return map[string]any{}, err
	}

	values := map[string]any{}
	for _, v := range dbValues {
		keys := helm.KeySplitHandleEscape(v.Key)
		value, err := helm.ParseValue(v.Value)
		if err != nil {
			return nil, err
		}
		helm.SetChartValue(keys, value, values)
	}

	return values, nil
}

func (a *Application) enrichWithTeamValues(ctx context.Context, values map[string]any) error {
	dbValues, err := a.repo.TeamValuesGet(ctx, a.chartType, a.team)
	if err != nil {
		return err
	}

	for _, v := range dbValues {
		err = parseTeamValue(v.Key, v.Value, values)
		if err != nil {
			return err
		}
	}

	return nil
}

func parseTeamValue(key string, value any, values map[string]any) error {
	keys := helm.KeySplitHandleEscape(key)

	if pKeys, cKeys, idx, mutate := isMutation(keys); mutate {
		return mutateGlobalListValue(pKeys, cKeys, idx, value, values)
	}

	value, err := helm.ParseValue(value)
	if err != nil {
		return err
	}
	helm.SetChartValue(keys, value, values)

	return nil
}

func isMutation(keys []string) ([]string, []string, int, bool) {
	for i, p := range keys {
		if idx, isListElement := isListElement(p); isListElement {
			return keys[:i], keys[i+1:], idx, true
		}
	}
	return []string{}, []string{}, 0, false
}

func isListElement(p string) (int, bool) {
	if strings.HasPrefix(p, "[") && strings.HasSuffix(p, "]") {
		v := strings.TrimPrefix(p, "[")
		v = strings.TrimSuffix(v, "]")
		idx, err := strconv.Atoi(v)
		if err != nil {
			return 0, false
		}
		return idx, true
	}

	return 0, false
}

func mutateGlobalListValue(pKeys, keys []string, idx int, value any, values map[string]any) error {
	parentList := findParentList(pKeys, values)
	value, err := helm.ParseValue(value)
	if err != nil {
		return err
	}
	helm.SetChartValue(keys, value, parentList[idx].(map[string]any))
	return nil
}

func findParentList(pKeys []string, values map[string]any) []any {
	key := pKeys[0]

	if len(pKeys) > 1 {
		return findParentList(pKeys[1:], values[key].(map[string]any))
	}

	return values[key].([]any)
}

func mergeMaps(base, custom map[string]any) map[string]any {
	for k, v := range custom {
		if _, ok := v.(map[string]any); ok {
			if _, ok := base[k].(map[string]any); !ok {
				base[k] = map[string]any{}
			}
			base[k] = mergeMaps(base[k].(map[string]any), v.(map[string]any))
			continue
		}
		base[k] = v
	}
	return base
}
