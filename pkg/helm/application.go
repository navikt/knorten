package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/utils/strings/slices"

	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/logger"
)

const (
	timeout = 30 * time.Minute
)

type HelmEventData struct {
	TeamID       string
	Namespace    string
	ReleaseName  string
	ChartType    gensql.ChartType
	ChartRepo    string
	ChartName    string
	ChartVersion string
}

type Client struct {
	dryRun bool
	repo   *database.Repo
}

func NewClient(dryRun bool, repo *database.Repo) Client {
	return Client{
		dryRun: dryRun,
		repo:   repo,
	}
}

func (c Client) InstallOrUpgrade(ctx context.Context, helmEvent HelmEventData, logger logger.Logger) error {
	logger.Infof("Installing or upgrading %v", helmEvent.ChartType)
	rollback, err := c.installOrUpgrade(ctx, helmEvent, logger)
	if rollback {
		switch helmEvent.ChartType {
		case gensql.ChartTypeJupyterhub:
			if err := c.repo.RegisterHelmRollbackJupyterEvent(context.Background(), helmEvent.TeamID, helmEvent); err != nil {
				logger.WithError(err).Error("registering helm rollback jupyter event")
			}
		case gensql.ChartTypeAirflow:
			if err := c.repo.RegisterHelmRollbackAirflowEvent(context.Background(), helmEvent.TeamID, helmEvent); err != nil {
				logger.WithError(err).Error("registering helm rollback airflow event")
			}
		}
	}
	if err != nil {
		logger.Infof("Installing or upgrading %v failed", helmEvent.ChartType)
		return err
	}

	logger.Infof("Successfully installed or upgraded %v", helmEvent.ChartType)
	return nil
}

func (c Client) installOrUpgrade(ctx context.Context, helmEvent HelmEventData, logger logger.Logger) (bool, error) {
	helmChart, err := c.createChartWithValues(ctx, helmEvent)
	if err != nil {
		logger.WithError(err).Error("getting chart values")
		return false, err
	}

	if c.dryRun {
		out, err := yaml.Marshal(helmChart.Values)
		if err != nil {
			logger.WithError(err).Error("marshalling team values")
			return false, err
		}

		if err = os.WriteFile(fmt.Sprintf("charts/%v-%v.yaml", helmEvent.ChartType, time.Now().Format("2006.01.02-15:04")), out, 0o644); err != nil {
			logger.WithError(err).Error("writing values to file")
			return true, err
		}

		return false, nil
	}

	settings := cli.New()
	settings.SetNamespace(helmEvent.Namespace)
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), "secret", log.Printf); err != nil {
		logger.WithError(err).Error("action config init")
		return false, err
	}

	exists, err := releaseExists(actionConfig, helmEvent.ReleaseName)
	if err != nil {
		logger.WithError(err).Error("checking if release exists")
		return false, err
	}

	if exists {
		upgradeClient := action.NewUpgrade(actionConfig)
		upgradeClient.Namespace = helmEvent.Namespace
		upgradeClient.Timeout = timeout

		_, err = upgradeClient.RunWithContext(ctx, helmEvent.ReleaseName, helmChart, helmChart.Values)
		if err != nil {
			logger.WithError(err).Error("helm upgrade")
			return true, err
		}
	} else {
		installClient := action.NewInstall(actionConfig)
		installClient.Namespace = helmEvent.Namespace
		installClient.ReleaseName = helmEvent.ReleaseName
		installClient.Timeout = timeout

		_, err = installClient.RunWithContext(ctx, helmChart, helmChart.Values)
		if err != nil {
			logger.WithError(err).Error("helm install")
			return false, err
		}
	}

	return false, nil
}

func (c Client) Uninstall(ctx context.Context, helmEvent HelmEventData, logger logger.Logger) bool {
	logger.Infof("Uninstalling %v", helmEvent.ChartType)
	if err := c.uninstall(ctx, helmEvent, logger); err != nil {
		logger.Infof("Uninstalling %v failed", helmEvent.ChartType)
		return true
	}

	logger.Infof("Successfully uninstalled %v", helmEvent.ChartType)
	return false
}

func (c Client) uninstall(ctx context.Context, helmEvent HelmEventData, logger logger.Logger) error {
	if c.dryRun {
		return nil
	}

	settings := cli.New()
	settings.SetNamespace(helmEvent.Namespace)
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), "secret", log.Printf); err != nil {
		logger.WithError(err).Errorf("creating action config for helm uninstall: release %v, team %v", helmEvent.TeamID, helmEvent.TeamID)
		return err
	}

	exists, err := releaseExists(actionConfig, helmEvent.ReleaseName)
	if err != nil {
		logger.WithError(err).Errorf("checking if release exists for helm uninstall: release %v, team %v", helmEvent.TeamID, helmEvent.TeamID)
		return err
	}

	if !exists {
		return nil
	}

	uninstallClient := action.NewUninstall(actionConfig)
	_, err = uninstallClient.Run(helmEvent.ReleaseName)
	if err != nil {
		logger.WithError(err).Errorf("helm uninstall: release %v, team %v", helmEvent.TeamID, helmEvent.TeamID)
		return err
	}

	return nil
}

func (c Client) Rollback(ctx context.Context, helmEvent HelmEventData, logger logger.Logger) (bool, error) {
	logger.Infof("Rolling back %v", helmEvent.ChartType)
	retry, err := c.rollback(ctx, helmEvent, logger)
	if retry || err != nil {
		logger.Infof("Rolling back %v failed", helmEvent.ChartType)
		return retry, err
	}

	logger.Infof("Successfully rolled back %v", helmEvent.ChartType)
	return false, nil
}

func (c Client) rollback(ctx context.Context, helmEvent HelmEventData, logger logger.Logger) (bool, error) {
	if c.dryRun {
		return false, nil
	}

	settings := cli.New()
	settings.SetNamespace(helmEvent.Namespace)
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), "secret", log.Printf); err != nil {
		logger.WithError(err).Error("action config init")
		return true, nil
	}

	version, err := lastSuccessfulHelmRelease(helmEvent.ReleaseName, actionConfig)
	if err != nil {
		logger.WithError(err).Errorf("unable to rollback chart %v for team %v", helmEvent.ChartName, helmEvent.TeamID)
		return false, err
	}

	rollbackClient := action.NewRollback(actionConfig)
	rollbackClient.Version = version
	if err := rollbackClient.Run(helmEvent.ReleaseName); err != nil {
		logger.WithError(err).Errorf("rolling back release %v for team %v to version %v", helmEvent.ReleaseName, helmEvent.TeamID, version)
		return true, nil
	}

	return false, nil
}

func lastSuccessfulHelmRelease(releaseName string, actionConfig *action.Configuration) (int, error) {
	historyClient := action.NewHistory(actionConfig)

	releases, err := historyClient.Run(releaseName)
	if err != nil {
		return 0, err
	}

	validStatuses := []string{release.StatusDeployed.String(), release.StatusSuperseded.String()}
	for i := len(releases) - 1; i >= 0; i-- {
		if slices.Contains(validStatuses, releases[i].Info.Status.String()) {
			return releases[i].Version, nil
		}
	}

	return 0, fmt.Errorf("no previous successful helm releases for %v", releaseName)
}

func (c Client) createChartWithValues(ctx context.Context, helmEvent HelmEventData) (*chart.Chart, error) {
	helmChart, err := FetchChart(helmEvent.ChartRepo, helmEvent.ChartName, helmEvent.ChartVersion)
	if err != nil {
		return nil, err
	}

	err = c.mergeValues(ctx, helmEvent.ChartType, helmEvent.TeamID, helmChart.Values)
	if err != nil {
		return nil, err
	}

	return helmChart, nil
}

func (c Client) mergeValues(ctx context.Context, chartType gensql.ChartType, teamID string, defaultValues map[string]any) error {
	values, err := c.globalValues(ctx, chartType)
	if err != nil {
		return err
	}

	err = c.enrichWithTeamValues(ctx, chartType, teamID, values)
	if err != nil {
		return err
	}

	switch chartType {
	case gensql.ChartTypeJupyterhub:
		if err := c.concatenateImageProfiles(ctx, teamID, values); err != nil {
			return err
		}
	case gensql.ChartTypeAirflow:
		knauditInitContainer, err := c.createKnauditInitContainer(ctx)
		if err != nil {
			return err
		}
		mergeMaps(values, knauditInitContainer)

		if err := c.concatenateCommonAirflowEnvs(ctx, teamID, values); err != nil {
			return err
		}
	}

	mergeMaps(defaultValues, values)
	return nil
}

func (c Client) globalValues(ctx context.Context, chartType gensql.ChartType) (map[string]any, error) {
	dbValues, err := c.repo.GlobalValuesGet(ctx, chartType)
	if err != nil {
		return map[string]any{}, err
	}

	values := map[string]any{}
	for _, v := range dbValues {
		if v.Encrypted {
			v.Value, err = c.repo.DecryptValue(v.Value)
			if err != nil {
				return nil, err
			}
		}

		keys := keySplitHandleEscape(v.Key)
		value, err := ParseValue(v.Value)
		if err != nil {
			return nil, err
		}
		SetChartValue(keys, value, values)
	}

	return values, nil
}

func (c Client) enrichWithTeamValues(ctx context.Context, chartType gensql.ChartType, teamID string, values map[string]any) error {
	dbValues, err := c.repo.TeamValuesGet(ctx, chartType, teamID)
	if err != nil {
		return err
	}

	for _, v := range dbValues {
		if slices.Contains([]string{"fernetKey", "webserverSecretKey"}, v.Key) {
			continue
		}

		_, err = parseTeamValue(v.Key, v.Value, values)
		if err != nil {
			return err
		}
	}

	return nil
}

func parseKey(key string) (string, []string) {
	opts := strings.Split(key, ",")
	return opts[0], opts[1:]
}

func parseTeamValue(key string, value any, values map[string]any) (any, error) {
	key, opts := parseKey(key)
	if slices.Contains(opts, "omit") {
		return nil, nil
	}

	keys := keySplitHandleEscape(key)
	value, err := ParseValue(value)
	if err != nil {
		return nil, err
	}
	SetChartValue(keys, value, values)

	return values, nil
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

func keySplitHandleEscape(key string) []string {
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
