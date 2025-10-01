package helm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/utils/strings/slices"

	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/database/gensql"
)

const (
	timeout = 30 * time.Minute
)

type EventData struct {
	TeamID       string
	Namespace    string
	ReleaseName  string
	ChartType    gensql.ChartType
	ChartRepo    string
	ChartName    string
	ChartVersion string
}

type Client struct {
	ops  Operations
	cfg  *Config
	repo *database.Repo
}

func NewClient(config *Config, repo *database.Repo) (*Client, error) {
	h := NewHelm(config)

	err := h.Update(context.Background())
	if err != nil {
		return nil, fmt.Errorf("updating helm repositories: %w", err)
	}

	return &Client{
		ops:  h,
		cfg:  config,
		repo: repo,
	}, nil
}

func (c *Client) InstallOrUpgrade(ctx context.Context, ev *EventData) error {
	// The enrichers are processed in the order they are added
	enrichers := []Enricher{
		NewGlobalEnricher(ev.ChartType, c.repo),
		NewTeamEnricher(ev.ChartType, ev.TeamID, c.repo),
	}

	switch ev.ChartType {
	case gensql.ChartTypeAirflow:
		enrichers = append(enrichers, NewAirflowEnricher(ev.TeamID, c.repo))
	}

	l := NewClassicLoader(
		ev.ChartRepo,
		ev.ChartName,
		ev.ChartVersion,
		c.ops,
		NewChainEnricher(enrichers...),
	)

	err := c.ops.Apply(ctx, l, &ApplyOpts{
		ReleaseName: ev.ReleaseName,
		Namespace:   ev.Namespace,
	})
	if err != nil {
		handleErrWithRollback(ctx, err, ev, c)

		return fmt.Errorf("installing or upgrading %v failed: %w", ev.ChartType, err)
	}

	return nil
}

// FIXME: Can we get rid of the switch at least, shouldn't be doing this here
func handleErrWithRollback(ctx context.Context, err error, helmEvent *EventData, c *Client) {
	var rollbackErr *ErrRollback
	if errors.As(err, &rollbackErr) {
		switch helmEvent.ChartType {
		case gensql.ChartTypeAirflow:
			_ = c.repo.RegisterHelmRollbackAirflowEvent(ctx, helmEvent.TeamID, helmEvent)
		}
	}
}

func (c *Client) Uninstall(ctx context.Context, helmEvent *EventData) error {
	err := c.ops.Delete(ctx, &DeleteOpts{
		ReleaseName: helmEvent.ReleaseName,
		Namespace:   helmEvent.Namespace,
	})
	if err != nil {
		return fmt.Errorf("uninstalling %v failed: %w", helmEvent.ChartType, err)
	}

	return nil
}

func (c *Client) Rollback(ctx context.Context, helmEvent *EventData) error {
	err := c.ops.Rollback(ctx, &RollbackOpts{
		ReleaseName: helmEvent.ReleaseName,
		Namespace:   helmEvent.Namespace,
	})
	if err != nil {
		return fmt.Errorf("rolling back %v failed: %w", helmEvent.ChartType, err)
	}

	return nil
}

func lastSuccessfulHelmRelease(
	releaseName string,
	actionConfig *action.Configuration,
) (int, error) {
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
