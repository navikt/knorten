package helm

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/navikt/knorten/pkg/database/gensql"
	"helm.sh/helm/v3/pkg/chart"
	"k8s.io/utils/strings/slices"
)

const (
	ProfileListKey  = "singleuser.profileList"
	EnvKey          = "env"
	KnauditImageKey = "knauditImage,omit"
)

type Enricher interface {
	Enrich(ctx context.Context, values map[string]any) (map[string]any, error)
}

type ClassicLoader struct {
	RepositoryName string
	ChartName      string
	Version        string

	fetcher  ChartFetcher
	enricher Enricher
}

var _ ChartLoader = &ClassicLoader{}

func (l *ClassicLoader) Load(ctx context.Context) (*chart.Chart, error) {
	ch, err := l.fetcher.Fetch(ctx, l.RepositoryName, l.ChartName, l.Version)
	if err != nil {
		return nil, fmt.Errorf("fetching chart: %w", err)
	}

	ch.Values, err = l.enricher.Enrich(ctx, ch.Values)
	if err != nil {
		return nil, fmt.Errorf("enriching values: %w", err)
	}

	return ch, nil
}

func NewClassicLoader(repositoryName, chartName, version string, fetcher ChartFetcher, enricher Enricher) *ClassicLoader {
	return &ClassicLoader{
		RepositoryName: repositoryName,
		ChartName:      chartName,
		Version:        version,
		fetcher:        fetcher,
		enricher:       enricher,
	}
}

type ChainEnricher struct {
	enrichers []Enricher
}

func (e *ChainEnricher) Enrich(ctx context.Context, values map[string]any) (map[string]any, error) {
	for _, enricher := range e.enrichers {
		var err error
		values, err = enricher.Enrich(ctx, values)
		if err != nil {
			return nil, fmt.Errorf("enriching values: %w", err)
		}
	}

	return values, nil
}

func NewChainEnricher(enrichers ...Enricher) *ChainEnricher {
	return &ChainEnricher{
		enrichers: enrichers,
	}
}

type GlobalEnricherStore interface {
	GlobalValuesGet(ctx context.Context, chartType gensql.ChartType) ([]gensql.ChartGlobalValue, error)
	DecryptValue(encValue string) (string, error)
}

type GlobalEnricher struct {
	chartType gensql.ChartType
	store     GlobalEnricherStore
}

func (g *GlobalEnricher) Enrich(ctx context.Context, values map[string]any) (map[string]any, error) {
	dbValues, err := g.store.GlobalValuesGet(ctx, g.chartType)
	if err != nil {
		return nil, fmt.Errorf("getting global values: %w", err)
	}

	globalValues := map[string]any{}

	for _, v := range dbValues {
		if v.Encrypted {
			v.Value, err = g.store.DecryptValue(v.Value)
			if err != nil {
				return nil, fmt.Errorf("decrypting value: %w", err)
			}
		}

		keys := keySplitHandleEscape(v.Key)

		value, err := ParseValue(v.Value)
		if err != nil {
			return nil, fmt.Errorf("parsing value: %w", err)
		}

		SetChartValue(keys, value, globalValues)
	}

	return mergeMaps(values, globalValues), nil
}

func NewGlobalEnricher(chartType gensql.ChartType, store GlobalEnricherStore) *GlobalEnricher {
	return &GlobalEnricher{
		chartType: chartType,
		store:     store,
	}
}

type TeamEnricherStore interface {
	TeamValuesGet(ctx context.Context, chartType gensql.ChartType, teamID string) ([]gensql.ChartTeamValue, error)
}

type TeamEnricher struct {
	chartType gensql.ChartType
	teamID    string
	store     TeamEnricherStore
}

func (e TeamEnricher) Enrich(ctx context.Context, values map[string]any) (map[string]any, error) {
	dbValues, err := e.store.TeamValuesGet(ctx, e.chartType, e.teamID)
	if err != nil {
		return nil, fmt.Errorf("getting team values: %w", err)
	}

	for _, v := range dbValues {
		if slices.Contains([]string{"fernetKey", "webserverSecretKey"}, v.Key) {
			continue
		}

		_, err = parseTeamValue(v.Key, v.Value, values)
		if err != nil {
			return nil, fmt.Errorf("parsing team value: %w", err)
		}
	}

	return values, nil
}

func NewTeamEnricher(chartType gensql.ChartType, teamID string, store TeamEnricherStore) *TeamEnricher {
	return &TeamEnricher{
		chartType: chartType,
		teamID:    teamID,
		store:     store,
	}
}

type JupyterhubEnricherStore interface {
	TeamValueGet(ctx context.Context, key, teamID string) (gensql.ChartTeamValue, error)
	GlobalValueGet(ctx context.Context, chartType gensql.ChartType, key string) (gensql.ChartGlobalValue, error)
}

type JupyterhubEnricher struct {
	teamID string
	store  JupyterhubEnricherStore
}

func (e *JupyterhubEnricher) Enrich(ctx context.Context, values map[string]any) (map[string]any, error) {
	var userProfiles []map[string]any

	userProfileList, err := e.store.TeamValueGet(ctx, ProfileListKey, e.teamID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("getting user profile list: %w", err)
	}

	if !errors.Is(err, sql.ErrNoRows) {
		err = json.Unmarshal([]byte(userProfileList.Value), &userProfiles)
		if err != nil {
			return nil, fmt.Errorf("unmarshalling user profile list: %w", err)
		}
	}

	globalProfileList, err := e.store.GlobalValueGet(ctx, gensql.ChartTypeJupyterhub, ProfileListKey)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("getting global profile list: %w", err)
	}

	var globalProfiles []map[string]any

	if !errors.Is(err, sql.ErrNoRows) {
		err = json.Unmarshal([]byte(globalProfileList.Value), &globalProfiles)
		if err != nil {
			return nil, fmt.Errorf("unmarshalling global profile list: %w", err)
		}
	}

	profiles := append(userProfiles, globalProfiles...)

	if len(profiles) > 0 {
		mergeMaps(values, map[string]any{
			"singleuser": map[string]any{
				"profileList": append(userProfiles, globalProfiles...),
			},
		})
	}

	return values, nil
}

func NewJupyterhubEnricher(teamID string, store JupyterhubEnricherStore) *JupyterhubEnricher {
	return &JupyterhubEnricher{
		store:  store,
		teamID: teamID,
	}
}

type AirflowEnricherStore interface {
	GlobalValueGet(ctx context.Context, chartType gensql.ChartType, key string) (gensql.ChartGlobalValue, error)
	TeamValueGet(ctx context.Context, key, teamID string) (gensql.ChartTeamValue, error)
}

type AirflowEnricher struct {
	teamID string
	store  AirflowEnricherStore
}

func (e *AirflowEnricher) Enrich(ctx context.Context, values map[string]any) (map[string]any, error) {
	knauditImage, err := e.store.GlobalValueGet(ctx, gensql.ChartTypeAirflow, KnauditImageKey)
	if err != nil {
		return nil, fmt.Errorf("getting knaudit image: %w", err)
	}

	image := map[string]any{
		"workers": map[string]any{
			"extraInitContainers": []map[string]any{
				{
					"name":  "knaudit",
					"image": knauditImage.Value,
					"env": []map[string]any{
						{
							"name":      "POD_NAME",
							"valueFrom": map[string]any{"fieldRef": map[string]string{"fieldPath": "metadata.name"}},
						},
						{
							"name":      "NAMESPACE",
							"valueFrom": map[string]any{"fieldRef": map[string]string{"fieldPath": "metadata.namespace"}},
						},
						{
							"name":  "KNAUDIT_PROXY_URL",
							"value": "http://knaudit-proxy.knada-system.svc.cluster.local",
						},
						{
							"name":  "CA_CERT_PATH",
							"value": "/etc/pki/tls/certs/ca-bundle.crt",
						},
						{
							"name":  "GIT_REPO_PATH",
							"value": "/dags",
						},
						{
							"name":      "AIRFLOW_DAG_ID",
							"valueFrom": map[string]any{"fieldRef": map[string]string{"fieldPath": "metadata.annotations['dag_id']"}},
						},
						{
							"name":      "AIRFLOW_RUN_ID",
							"valueFrom": map[string]any{"fieldRef": map[string]string{"fieldPath": "metadata.annotations['run_id']"}},
						},
						{
							"name":      "AIRFLOW_TASK_ID",
							"valueFrom": map[string]any{"fieldRef": map[string]string{"fieldPath": "metadata.annotations['task_id']"}},
						},
						{
							"name":      "AIRFLOW_DB_URL",
							"valueFrom": map[string]any{"secretKeyRef": map[string]string{"name": "airflow-db", "key": "connection"}},
						},
					},
					"resources": map[string]any{
						"requests": map[string]string{
							"cpu":    "200m",
							"memory": "128Mi",
						},
					},
					"volumeMounts": []map[string]any{
						{
							"mountPath": "/dags",
							"name":      "dags",
						},
						{
							"mountPath": "/etc/pki/tls/certs/ca-bundle.crt",
							"name":      "ca-bundle-pem",
							"readOnly":  true,
							"subPath":   "ca-bundle.pem",
						},
					},
				},
			},
		},
	}

	values = mergeMaps(values, image)

	globalEnvsSQL, err := e.store.GlobalValueGet(ctx, gensql.ChartTypeAirflow, EnvKey)
	if err != nil {
		return nil, fmt.Errorf("getting global envs: %w", err)
	}

	var globalEnvs []map[string]string
	if err := json.Unmarshal([]byte(globalEnvsSQL.Value), &globalEnvs); err != nil {
		return nil, fmt.Errorf("unmarshalling global envs: %w", err)
	}

	teamEnvsSQL, err := e.store.TeamValueGet(ctx, EnvKey, e.teamID)
	if err != nil {
		return nil, fmt.Errorf("getting team envs: %w", err)
	}

	var teamEnvs []map[string]string
	if err := json.Unmarshal([]byte(teamEnvsSQL.Value), &teamEnvs); err != nil {
		return nil, fmt.Errorf("unmarshalling team envs: %w", err)
	}

	envs := append(globalEnvs, teamEnvs...)

	if len(envs) > 0 {
		values = mergeMaps(values, map[string]any{
			EnvKey: append(globalEnvs, teamEnvs...),
		})
	}

	return values, nil
}

func NewAirflowEnricher(teamID string, store AirflowEnricherStore) *AirflowEnricher {
	return &AirflowEnricher{
		teamID: teamID,
		store:  store,
	}
}

type DeclarativeLoader struct {
	chart *Chart
}

func (d *DeclarativeLoader) Load(_ context.Context) (map[string]any, error) {
	marshaller, ok := d.chart.Values.(Marshaller)
	if !ok {
		return nil, fmt.Errorf("values is not a marshaller")
	}

	rawValues, err := marshaller.MarshalYAML()
	if err != nil {
		return nil, fmt.Errorf("marshalling values: %w", err)
	}

	values, err := UnmarshalToValues(rawValues)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling values: %w", err)
	}

	return values, nil
}
