package database

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/navikt/knorten/pkg/database/gensql"
	"github.com/navikt/knorten/pkg/reflect"
)

func (r *Repo) TeamCreate(ctx context.Context, team gensql.Team) error {
	return r.querier.TeamCreate(ctx, gensql.TeamCreateParams{
		ID:    team.ID,
		Users: stringSliceToLower(team.Users),
		Slug:  team.Slug,
	})
}

func (r *Repo) TeamUpdate(ctx context.Context, team gensql.Team) error {
	return r.querier.TeamUpdate(ctx, gensql.TeamUpdateParams{
		ID:    team.ID,
		Users: stringSliceToLower(team.Users),
	})
}

func (r *Repo) TeamGet(ctx context.Context, teamID string) (gensql.TeamGetRow, error) {
	return r.querier.TeamGet(ctx, teamID)
}

func (r *Repo) TeamBySlugGet(ctx context.Context, slug string) (gensql.TeamBySlugGetRow, error) {
	return r.querier.TeamBySlugGet(ctx, slug)
}

func (r *Repo) TeamDelete(ctx context.Context, teamID string) error {
	return r.querier.TeamDelete(ctx, teamID)
}

func (r *Repo) TeamsGet(ctx context.Context) ([]gensql.Team, error) {
	return r.querier.TeamsGet(ctx)
}

func (r *Repo) TeamsForChartGet(ctx context.Context, chartType gensql.ChartType) ([]string, error) {
	return r.querier.TeamsForChartGet(ctx, chartType)
}

func (r *Repo) TeamsForUser(ctx context.Context, email string) ([]string, error) {
	teams, err := r.querier.TeamsForUserGet(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []string{}, nil
		}
		return nil, err
	}

	var teamList []string
	for _, t := range teams {
		teamList = append(teamList, t.ID)
	}

	return teamList, nil
}

func (r *Repo) TeamChartValueInsert(ctx context.Context, key, value, teamID string, chartType gensql.ChartType) error {
	return r.querier.TeamValueInsert(ctx, gensql.TeamValueInsertParams{
		Key:       key,
		Value:     value,
		TeamID:    teamID,
		ChartType: chartType,
	})
}

func (r *Repo) TeamValuesGet(ctx context.Context, chartType gensql.ChartType, teamID string) ([]gensql.ChartTeamValue, error) {
	return r.querier.TeamValuesGet(ctx, gensql.TeamValuesGetParams{
		ChartType: chartType,
		TeamID:    teamID,
	})
}

func (r *Repo) TeamValueGet(ctx context.Context, key, teamID string) (gensql.ChartTeamValue, error) {
	return r.querier.TeamValueGet(ctx, gensql.TeamValueGetParams{
		Key:    key,
		TeamID: teamID,
	})
}

func (r *Repo) TeamValueDelete(ctx context.Context, key, teamID string) error {
	return r.querier.TeamValueDelete(ctx, gensql.TeamValueDeleteParams{
		Key:    key,
		TeamID: teamID,
	})
}

func (r *Repo) TeamConfigurableValuesGet(ctx context.Context, chartType gensql.ChartType, teamID string, obj any) error {
	teamValues, err := r.querier.TeamValuesGet(ctx, gensql.TeamValuesGetParams{
		ChartType: chartType,
		TeamID:    teamID,
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
