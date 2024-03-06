package mock

import (
	"context"
	"github.com/navikt/knorten/pkg/database/gensql"
	"github.com/navikt/knorten/pkg/helm"
)

type EnricherStore struct {
	GlobalValuesGetFn func(ctx context.Context, chartType gensql.ChartType) ([]gensql.ChartGlobalValue, error)
	GlobalValueGetFn  func(ctx context.Context, chartType gensql.ChartType, key string) (gensql.ChartGlobalValue, error)
	TeamValueGetFn    func(ctx context.Context, key, teamID string) (gensql.ChartTeamValue, error)
	TeamValuesGetFn   func(ctx context.Context, chartType gensql.ChartType, teamID string) ([]gensql.ChartTeamValue, error)
	DecryptValueFn    func(encValue string) (string, error)

	globalValues map[string]gensql.ChartGlobalValue
	teamValues   map[string]gensql.ChartTeamValue
}

// We should probably have a mock for each of the interfaces in the helm package, but
// for now we will make it a little simpler, and just create one store mock for all enrichers
var _ helm.GlobalEnricherStore = &EnricherStore{}
var _ helm.TeamEnricherStore = &EnricherStore{}
var _ helm.AirflowEnricherStore = &EnricherStore{}
var _ helm.JupyterhubEnricherStore = &EnricherStore{}

func (s *EnricherStore) GlobalValuesGet(ctx context.Context, chartType gensql.ChartType) ([]gensql.ChartGlobalValue, error) {
	return s.GlobalValuesGetFn(ctx, chartType)
}

func (s *EnricherStore) GlobalValueGet(ctx context.Context, chartType gensql.ChartType, key string) (gensql.ChartGlobalValue, error) {
	return s.GlobalValueGetFn(ctx, chartType, key)
}

func (s *EnricherStore) TeamValueGet(ctx context.Context, key, teamID string) (gensql.ChartTeamValue, error) {
	return s.TeamValueGetFn(ctx, key, teamID)
}

func (s *EnricherStore) TeamValuesGet(ctx context.Context, chartType gensql.ChartType, teamID string) ([]gensql.ChartTeamValue, error) {
	return s.TeamValuesGetFn(ctx, chartType, teamID)
}

func (s *EnricherStore) DecryptValue(encValue string) (string, error) {
	return s.DecryptValueFn(encValue)
}

func (e *EnricherStore) SetGlobalValue(key string, value gensql.ChartGlobalValue) *EnricherStore {
	e.globalValues[key] = value
	return e
}

func (e *EnricherStore) SetTeamValue(key string, value gensql.ChartTeamValue) *EnricherStore {
	e.teamValues[key] = value
	return e
}

func NewEnricherStore(decryptValue *string, globalValue *gensql.ChartGlobalValue, teamValue *gensql.ChartTeamValue, err error) *EnricherStore {
	e := &EnricherStore{
		globalValues: map[string]gensql.ChartGlobalValue{},
		teamValues:   map[string]gensql.ChartTeamValue{},
	}

	e.GlobalValuesGetFn = func(_ context.Context, chartType gensql.ChartType) ([]gensql.ChartGlobalValue, error) {
		if globalValue == nil {
			return nil, err
		}

		return []gensql.ChartGlobalValue{*globalValue}, err
	}

	e.GlobalValueGetFn = func(_ context.Context, chartType gensql.ChartType, key string) (gensql.ChartGlobalValue, error) {
		return e.globalValues[key], err
	}

	e.TeamValueGetFn = func(_ context.Context, key, teamID string) (gensql.ChartTeamValue, error) {
		return e.teamValues[key], err
	}

	e.TeamValuesGetFn = func(_ context.Context, chartType gensql.ChartType, teamID string) ([]gensql.ChartTeamValue, error) {
		if teamValue == nil {
			return nil, err
		}

		return []gensql.ChartTeamValue{*teamValue}, err
	}

	e.DecryptValueFn = func(_ string) (string, error) {
		if decryptValue == nil {
			return "", err
		}

		return *decryptValue, err
	}

	return e
}
