package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/helm"
	"helm.sh/helm/v3/pkg/chart"
)

type Jupyterhub struct {
	team string
	repo *database.Repo
}

func NewJupyterhub(team string, repo *database.Repo) *Jupyterhub {
	return &Jupyterhub{
		team: team,
		repo: repo,
	}
}

func (j *Jupyterhub) Chart(ctx context.Context) (*chart.Chart, error) {
	chart, err := helm.FetchChart("jupyterhub", "0.11.1", "https://jupyterhub.github.io/helm-chart")
	if err != nil {
		return nil, err
	}

	err = j.mergeValues(ctx, chart.Values)
	if err != nil {
		return nil, err
	}

	return chart, nil
}

func (j *Jupyterhub) mergeValues(ctx context.Context, defaultValues map[string]any) error {
	values, err := j.globalValues(ctx)
	if err != nil {
		return err
	}

	values, err = j.enrichWithTeamValues(ctx, values)
	if err != nil {
		return err
	}

	for key, value := range values {
		keyPath := keySplitHandleEscape(key)
		setChartValue(keyPath, value, defaultValues)
	}

	return nil
}

func (j *Jupyterhub) globalValues(ctx context.Context) (map[string]any, error) {
	dbValues, err := j.repo.GlobalValuesGet(ctx, gensql.ChartTypeJupyterhub)
	if err != nil {
		return map[string]any{}, err
	}

	values := map[string]any{}
	for _, v := range dbValues {
		values[v.Key], err = parseValue(v.Value)
		if err != nil {
			return nil, err
		}
	}

	return values, nil
}

func (j *Jupyterhub) enrichWithTeamValues(ctx context.Context, values map[string]any) (map[string]any, error) {
	dbValues, err := j.repo.TeamValuesGet(ctx, gensql.ChartTypeJupyterhub, j.team)
	if err != nil {
		return map[string]any{}, err
	}

	for _, v := range dbValues {
		values[v.Key], err = parseValue(v.Value)
		if err != nil {
			return nil, err
		}
	}

	return values, nil
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

	keysWithoutEscape := []string{}
	for _, k := range keys {
		keysWithoutEscape = append(keysWithoutEscape, strings.ReplaceAll(k, "\\", ""))
	}

	return keysWithoutEscape
}

func setChartValue(keys []string, value any, chart map[string]any) {
	key := keys[0]
	if len(keys) > 1 {
		if _, ok := chart[key].(map[string]any); !ok {
			chart[key] = map[string]any{}
		}
		setChartValue(keys[1:], value, chart[key].(map[string]any))
		return
	}

	chart[key] = value
}

func parseValue(value any) (any, error) {
	var err error

	switch v := value.(type) {
	case string:
		value, err = parseString(v)
		if err != nil {
			fmt.Println("parsing value", v)
			return nil, err
		}
	default:
		value = v
	}

	return value, nil
}

func parseString(value any) (any, error) {
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

	return valueString, nil
}
