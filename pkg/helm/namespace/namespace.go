package helm

import (
	"context"

	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/helm"
	"helm.sh/helm/v3/pkg/chart"
)

type Namespace struct {
	team string
	repo *database.Repo
}

func NewNamespace(team string, repo *database.Repo) *Namespace {
	return &Namespace{
		team: team,
		repo: repo,
	}
}

func (n *Namespace) Chart(ctx context.Context) (*chart.Chart, error) {
	chart, err := helm.FetchChart("oci://europe-west1-docker.pkg.dev/knada-gcp/helm", "knada-namespace-setup", "0.1.3")
	if err != nil {
		return nil, err
	}

	err = n.mergeValues(ctx, chart.Values)
	if err != nil {
		return nil, err
	}

	return chart, nil
}

func (n *Namespace) mergeValues(ctx context.Context, defaultValues map[string]any) error {
	values, err := n.globalValues(ctx)
	if err != nil {
		return err
	}

	values, err = n.enrichWithTeamValues(ctx, values)
	if err != nil {
		return err
	}

	for key, value := range values {
		keyPath := helm.KeySplitHandleEscape(key)
		helm.SetChartValue(keyPath, value, defaultValues)
	}

	return nil
}

func (n *Namespace) globalValues(ctx context.Context) (map[string]any, error) {
	dbValues, err := n.repo.GlobalValuesGet(ctx, gensql.ChartTypeNamespace)
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

func (n *Namespace) enrichWithTeamValues(ctx context.Context, values map[string]any) (map[string]any, error) {
	dbValues, err := n.repo.TeamValuesGet(ctx, gensql.ChartTypeJupyterhub, n.team)
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
