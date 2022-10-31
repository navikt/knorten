package helm

import (
	"bytes"
	"context"
	"html/template"
	"os"
	"reflect"

	"github.com/knadh/koanf/maps"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"gopkg.in/yaml.v2"
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

	values, err := j.customValues(ctx)
	if err != nil {
		return nil, err
	}

	maps.IntfaceKeysToStrings(values)
	maps.IntfaceKeysToStrings(chart.Values)
	maps.Merge(values, chart.Values)
	return chart, nil
}

func (j *Jupyterhub) customValues(ctx context.Context) (map[string]interface{}, error) {
	gVals, err := j.globalValues(ctx)
	if err != nil {
		return nil, err
	}

	tVals, err := j.teamValues(ctx)
	if err != nil {
		return nil, err
	}

	vals := JupyterValues{
		gVals,
		tVals,
	}

	dataBytes, err := os.ReadFile(j.templatePath)
	if err != nil {
		return nil, err
	}
	tmpl, err := template.New("jupyterhub").Parse(string(dataBytes))
	if err != nil {
		return nil, err
	}

	buffer := &bytes.Buffer{}
	if err := tmpl.Execute(buffer, vals); err != nil {
		return nil, err
	}

	var values map[string]interface{}
	if err := yaml.Unmarshal(buffer.Bytes(), &values); err != nil {
		return nil, err
	}

	return values, nil
}

func (j *Jupyterhub) globalValues(ctx context.Context) (JupyterGlobalValues, error) {
	dbValues, err := j.repo.GlobalValuesGet(ctx, gensql.ChartTypeJupyterhub)
	if err != nil {
		return JupyterGlobalValues{}, err
	}

	values := JupyterGlobalValues{}
	for _, v := range dbValues {
		field := reflect.ValueOf(&values).Elem().FieldByName(v.Key)
		if field.CanSet() {
			field.SetString(v.Value)
		}
	}

	return values, nil
}

func (j *Jupyterhub) teamValues(ctx context.Context) (JupyterTeamValues, error) {
	dbValues, err := j.repo.TeamValuesGet(ctx, gensql.ChartTypeJupyterhub, j.team)
	if err != nil {
		return JupyterTeamValues{}, err
	}

	values := JupyterTeamValues{}
	for _, v := range dbValues {
		field := reflect.ValueOf(&values).Elem().FieldByName(v.Key)
		if field.CanSet() {
			field.SetString(v.Value)
		}
	}

	return values, nil
}
