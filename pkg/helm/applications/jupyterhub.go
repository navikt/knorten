package helm

import (
	"context"
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
		values[v.Key] = parseValue(v.Value)
	}

	return values, nil
}

func (j *Jupyterhub) enrichWithTeamValues(ctx context.Context, values map[string]any) (map[string]any, error) {
	dbValues, err := j.repo.TeamValuesGet(ctx, gensql.ChartTypeJupyterhub, j.team)
	if err != nil {
		return map[string]any{}, err
	}

	for _, v := range dbValues {
		values[v.Key] = parseValue(v.Value)
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

func parseValue(value any) any {
	valueStr := value.(string)
	if strings.HasPrefix(valueStr, "[") {
		listItems := splitString(valueStr[1:len(valueStr)-1], ',')
		value = []any{}
		for _, i := range listItems {
			value = append(value.([]any), parseValue(i))
		}
	} else if strings.HasPrefix(valueStr, "{") {
		mapItems := splitString(valueStr[1:len(valueStr)-1], ',')
		value = map[string]any{}
		for _, i := range mapItems {
			kv := splitString(i, ':')
			if len(kv) != 2 {
				panic("invalid map format")
			}
			value.(map[string]any)[kv[0]] = parseValue(kv[1])
		}
	}

	return value
}

func splitString(value string, sep rune) []string {
	mCount := 0
	sCount := 0
	items := strings.FieldsFunc(value, func(r rune) bool {
		switch r {
		case sep:
			return sCount == 0 && mCount == 0
		case '{':
			mCount += 1
		case '[':
			sCount += 1
		case '}':
			mCount -= 1
		case ']':
			sCount -= 1
		}
		return false
	})

	return items
}
