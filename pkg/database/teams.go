package database

import (
	"context"
	"strings"

	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/reflect"
)

func (r *Repo) TeamCreate(ctx context.Context, team gensql.Team) error {
	return r.querier.TeamCreate(ctx, gensql.TeamCreateParams{
		ID:        team.ID,
		Users:     stringSliceToLower(team.Users),
		Slug:      team.Slug,
		ApiAccess: team.ApiAccess,
		Owner:     team.Owner,
	})
}

func (r *Repo) TeamUpdate(ctx context.Context, team gensql.Team) error {
	return r.querier.TeamUpdate(ctx, gensql.TeamUpdateParams{
		ID:        team.ID,
		Users:     stringSliceToLower(team.Users),
		ApiAccess: team.ApiAccess,
	})
}

func (r *Repo) TeamGet(ctx context.Context, slug string) (gensql.TeamGetRow, error) {
	team, err := r.querier.TeamGet(ctx, slug)
	if err != nil {
		return gensql.TeamGetRow{}, err
	}
	team.Users = append(team.Users, team.Owner)
	return team, nil
}

func (r *Repo) TeamDelete(ctx context.Context, team string) error {
	return r.querier.TeamDelete(ctx, team)
}

func (r *Repo) TeamsGet(ctx context.Context) ([]gensql.Team, error) {
	return r.querier.TeamsGet(ctx)
}

func (r *Repo) TeamsForAppGet(ctx context.Context, chartType gensql.ChartType) ([]string, error) {
	return r.querier.TeamsForAppGet(ctx, chartType)
}

func (r *Repo) TeamSetPendingUpgrade(ctx context.Context, teamID, chartType string, pendingUpgrade bool) error {
	var err error
	switch chartType {
	case string(gensql.ChartTypeJupyterhub):
		err = r.querier.TeamSetPendingJupyterUpgrade(ctx, gensql.TeamSetPendingJupyterUpgradeParams{
			ID:                    teamID,
			PendingJupyterUpgrade: pendingUpgrade,
		})
	case string(gensql.ChartTypeAirflow):
		err = r.querier.TeamSetPendingAirflowUpgrade(ctx, gensql.TeamSetPendingAirflowUpgradeParams{
			ID:                    teamID,
			PendingAirflowUpgrade: pendingUpgrade,
		})
	}
	return err
}

func (r *Repo) TeamSetRestrictAirflowEgress(ctx context.Context, teamID string, restrictAirflowEgress bool) error {
	return r.querier.TeamSetAirflowRestrictEgress(ctx, gensql.TeamSetAirflowRestrictEgressParams{
		RestrictAirflowEgress: restrictAirflowEgress,
		ID:                    teamID,
	})
}

func (r *Repo) TeamSetApiAccess(ctx context.Context, teamID string, apiAccess bool) error {
	return r.querier.TeamSetApiAccess(ctx, gensql.TeamSetApiAccessParams{
		ApiAccess: apiAccess,
		ID:        teamID,
	})
}

func (r *Repo) TeamChartValueInsert(ctx context.Context, key, value, team string, chartType gensql.ChartType) error {
	return r.querier.TeamValueInsert(ctx, gensql.TeamValueInsertParams{
		Key:       key,
		Value:     value,
		TeamID:    team,
		ChartType: chartType,
	})
}

func (r *Repo) TeamValuesGet(ctx context.Context, chartType gensql.ChartType, team string) ([]gensql.ChartTeamValue, error) {
	return r.querier.TeamValuesGet(ctx, gensql.TeamValuesGetParams{
		ChartType: chartType,
		TeamID:    team,
	})
}

func (r *Repo) TeamValueGet(ctx context.Context, key, team string) (gensql.ChartTeamValue, error) {
	return r.querier.TeamValueGet(ctx, gensql.TeamValueGetParams{
		Key:    key,
		TeamID: team,
	})
}

func (r *Repo) TeamValueDelete(ctx context.Context, key, team string) error {
	return r.querier.TeamValueDelete(ctx, gensql.TeamValueDeleteParams{
		Key:    key,
		TeamID: team,
	})
}

func (r *Repo) TeamConfigurableValuesGet(ctx context.Context, chartType gensql.ChartType, team string, obj any) error {
	teamValues, err := r.querier.TeamValuesGet(ctx, gensql.TeamValuesGetParams{
		ChartType: chartType,
		TeamID:    team,
	})
	if err != nil {
		return err
	}

	values := map[string]string{}
	for _, value := range teamValues {
		values[value.Key] = value.Value
	}

	return reflect.InterfaceToStruct(obj, values)
}

func stringSliceToLower(vals []string) []string {
	var out []string
	for _, v := range vals {
		out = append(out, strings.ToLower(v))
	}

	return out
}
