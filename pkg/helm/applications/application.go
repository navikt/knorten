package helm

import (
	"context"
	"strconv"
	"strings"

	"helm.sh/helm/v3/pkg/chart"

	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/crypto"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/helm"
)

type Application struct {
	chartName    string
	chartRepo    string
	chartType    gensql.ChartType
	chartVersion string
	teamID       string
	repo         *database.Repo
	cryptClient  *crypto.EncrypterDecrypter
}

// TODO: Vi bør ta inn chart-settings som config

func NewAirflow(teamID string, repo *database.Repo, cryptClient *crypto.EncrypterDecrypter, chartVersion string) *Application {
	return &Application{
		chartName:    "airflow",
		chartRepo:    "apache-airflow",
		chartType:    gensql.ChartTypeAirflow,
		chartVersion: chartVersion,
		teamID:       teamID,
		repo:         repo,
		cryptClient:  cryptClient,
	}
}

func NewJupyterhub(teamID string, repo *database.Repo, cryptClient *crypto.EncrypterDecrypter, chartVersion string) *Application {
	return &Application{
		chartName:    "jupyterhub",
		chartRepo:    "jupyterhub",
		chartType:    gensql.ChartTypeJupyterhub,
		chartVersion: chartVersion,
		teamID:       teamID,
		repo:         repo,
		cryptClient:  cryptClient,
	}
}

func (a *Application) Chart(ctx context.Context) (*chart.Chart, error) {
	charty, err := helm.FetchChart(a.chartRepo, a.chartName, a.chartVersion)
	if err != nil {
		return nil, err
	}

	err = a.mergeValues(ctx, charty.Values)
	if err != nil {
		return nil, err
	}

	return charty, nil
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
		if v.Encrypted {
			v.Value, err = a.cryptClient.DecryptValue(v.Value)
			if err != nil {
				return nil, err
			}
		}
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
	dbValues, err := a.repo.TeamValuesGet(ctx, a.chartType, a.teamID)
	if err != nil {
		return err
	}

	for _, v := range dbValues {
		_, err = parseTeamValue(v.Key, v.Value, values)
		if err != nil {
			return err
		}
	}

	return nil
}

func parseTeamValue(key string, value any, values map[string]any) (any, error) {
	keys := helm.KeySplitHandleEscape(key)

	if pKeys, cKeys, idx, mutate := isMutation(keys); mutate {
		return mutateGlobalListValue(pKeys, cKeys, idx, value, values)
	}

	value, err := helm.ParseValue(value)
	if err != nil {
		return nil, err
	}
	helm.SetChartValue(keys, value, values)

	return values, nil
}

func isMutation(keys []string) ([]string, string, int, bool) {
	for i, p := range keys {
		if idx, isListElement := isListElement(p); isListElement {
			return keys[:i], strings.Join(keys[i+1:], "."), idx, true
		}
	}
	return []string{}, "", 0, false
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

func mutateGlobalListValue(pKeys []string, key string, idx int, value any, values map[string]any) (any, error) {
	parentList := findParentList(pKeys, values)
	value, err := helm.ParseValue(value)
	if err != nil {
		return nil, err
	}

	if parent, ok := parentList[idx].(map[string]any); ok {
		parent, err := parseTeamValue(key, value, parent)
		if err != nil {
			return nil, err
		}

		return parent, nil
	}
	parentList[idx] = value
	return parentList[idx], nil
}

func findParentList(pKeys []string, values map[string]any) []any {
	key := pKeys[0]

	if len(pKeys) > 1 {
		// fmt.Printf("pKeys: %v\n", pKeys)
		// fmt.Printf("values: %v\n", values)
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
