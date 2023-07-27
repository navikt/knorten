package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"

	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
)

const (
	timeout = 30 * time.Minute
)

type Application struct {
	chartName    string
	chartRepo    string
	chartType    gensql.ChartType
	chartVersion string
	teamID       string
	repo         *database.Repo
}

func newApplication(chartName, chartRepo, teamID, chartVersion string, chartType gensql.ChartType, repo *database.Repo) *Application {
	return &Application{
		chartName:    chartName,
		chartRepo:    chartRepo,
		chartType:    chartType,
		chartVersion: chartVersion,
		teamID:       teamID,
		repo:         repo,
	}
}

func InstallOrUpgrade(ctx context.Context, releaseName, namespace, teamID, chartName, chartRepo, chartVersion string, chartType gensql.ChartType, repo *database.Repo) error {
	application := newApplication(chartName, chartRepo, teamID, chartVersion, chartType, repo)
	teamValues, err := application.chartValues(ctx)
	if err != nil {
		return err
	}

	settings := cli.New()
	settings.SetNamespace(namespace)
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), "secret", log.Printf); err != nil {
		return err
	}

	var charty *chart.Chart
	switch chartType {
	case gensql.ChartTypeJupyterhub:
		charty, err = FetchChart("jupyterhub", "jupyterhub", chartVersion)
	case gensql.ChartTypeAirflow:
		charty, err = FetchChart("apache-airflow", "airflow", chartVersion)
	default:
		return fmt.Errorf("chart type for release %v is not supported", releaseName)
	}
	if err != nil {
		return err
	}

	charty.Values = teamValues

	exists, err := releaseExists(actionConfig, releaseName)
	if err != nil {
		return err
	}

	if exists {
		upgradeClient := action.NewUpgrade(actionConfig)
		upgradeClient.Namespace = namespace
		upgradeClient.Timeout = timeout

		// upgradeClient.Atomic = true
		// Fra doc: The --wait flag will be set automatically if --atomic is used.
		// Dette hindrer post-upgrade hooken som trigger databasemigrasjonsjobben for airflow og dermed blir alle airflow tjenester låst i wait-for-migrations initcontaineren når
		// vi bumper til ny versjon av airflow hvis denne krever db migrasjoner. Tenker vi løser dette annerledes uansett når vi går over til pubsub så kommenterer det ut for nå.

		_, err = upgradeClient.Run(releaseName, charty, charty.Values)
		if err != nil {
			return err
		}
	} else {
		installClient := action.NewInstall(actionConfig)
		installClient.Namespace = namespace
		installClient.ReleaseName = releaseName
		installClient.Timeout = timeout

		_, err = installClient.Run(charty, charty.Values)
		if err != nil {
			return err
		}
	}

	return nil
}

func Uninstall(releaseName, namespace string) error {
	settings := cli.New()
	settings.SetNamespace(namespace)
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), "secret", log.Printf); err != nil {
		return err
	}

	exists, err := releaseExists(actionConfig, releaseName)
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	uninstallClient := action.NewUninstall(actionConfig)
	_, err = uninstallClient.Run(releaseName)
	if err != nil {
		return err
	}

	return nil
}

func (a *Application) chartValues(ctx context.Context) (map[string]any, error) {
	charty, err := FetchChart(a.chartRepo, a.chartName, a.chartVersion)
	if err != nil {
		return nil, err
	}

	err = a.mergeValues(ctx, charty.Values)
	if err != nil {
		return nil, err
	}

	return charty.Values, nil
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
			v.Value, err = a.repo.DecryptValue(v.Value)
			if err != nil {
				return nil, err
			}
		}
		keys := KeySplitHandleEscape(v.Key)
		value, err := ParseValue(v.Value)
		if err != nil {
			return nil, err
		}
		SetChartValue(keys, value, values)
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
	keys := KeySplitHandleEscape(key)

	if pKeys, cKeys, idx, mutate := isMutation(keys); mutate {
		return mutateGlobalListValue(pKeys, cKeys, idx, value, values)
	}

	value, err := ParseValue(value)
	if err != nil {
		return nil, err
	}
	SetChartValue(keys, value, values)

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
	value, err := ParseValue(value)
	if err != nil {
		return nil, err
	}

	if len(parentList) == 0 {
		parentList = append(parentList, map[string]any{})
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
		_, ok := values[key].(map[string]any)
		if !ok {
			values[key] = map[string]any{}
		}

		return findParentList(pKeys[1:], values[key].(map[string]any))
	}

	_, ok := values[key].([]any)
	if !ok {
		values[key] = []any{}
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

func releaseExists(actionConfig *action.Configuration, releaseName string) (bool, error) {
	listClient := action.NewList(actionConfig)
	listClient.Deployed = true
	results, err := listClient.Run()
	if err != nil {
		return false, err
	}

	for _, r := range results {
		if r.Name == releaseName {
			return true, nil
		}
	}

	return false, nil
}

func KeySplitHandleEscape(key string) []string {
	escape := false
	keys := strings.FieldsFunc(key, func(r rune) bool {
		if r == '\\' {
			escape = true
		} else if escape {
			escape = false
			return false
		}
		return r == '.'
	})

	var keysWithoutEscape []string
	for _, k := range keys {
		keysWithoutEscape = append(keysWithoutEscape, strings.ReplaceAll(k, "\\", ""))
	}

	return keysWithoutEscape
}

func SetChartValue(keys []string, value any, chart map[string]any) {
	key := keys[0]
	if len(keys) > 1 {
		if _, ok := chart[key].(map[string]any); !ok {
			chart[key] = map[string]any{}
		}
		SetChartValue(keys[1:], value, chart[key].(map[string]any))
		return
	}

	chart[key] = value
}

func ParseValue(value any) (any, error) {
	var err error

	switch v := value.(type) {
	case string:
		value, err = ParseString(v)
		if err != nil {
			return nil, fmt.Errorf("failed parsing value %v: %v", v, err)
		}
	default:
		value = v
	}

	return value, nil
}

func ParseString(value any) (any, error) {
	valueString := value.(string)

	if d, err := strconv.ParseBool(valueString); err == nil {
		return d, nil
	} else if d, err := strconv.ParseInt(valueString, 10, 64); err == nil {
		return d, nil
	} else if d, err := strconv.ParseFloat(valueString, 64); err == nil {
		return d, nil
	} else if strings.HasPrefix(value.(string), "[") || strings.HasPrefix(value.(string), "{") {
		var d any
		if err := json.Unmarshal([]byte(valueString), &d); err != nil {
			return nil, err
		}
		return d, nil
	}

	return removeQuotations(valueString), nil
}

func removeQuotations(s string) string {
	s = strings.TrimPrefix(s, "\"")
	return strings.TrimSuffix(s, "\"")
}
