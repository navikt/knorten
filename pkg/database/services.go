package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/navikt/knorten/pkg/database/gensql"
	"github.com/navikt/knorten/pkg/k8s"
	"golang.org/x/exp/slices"
)

type AppService struct {
	App       string
	Ingress   string
	Slug      string
	Namespace string
}

type TeamServices struct {
	TeamID     string
	Slug       string
	Jupyterhub *AppService
	Airflow    *AppService
	Events     []EventWithLogs
}

type UserServices struct {
	Services   []TeamServices
	UserGSM    *gensql.UserGoogleSecretManager
	UserEvents []EventWithLogs
}

func createIngress(team string, chartType gensql.ChartType, topLevelDomain string) string {
	switch chartType {
	case gensql.ChartTypeJupyterhub:
		return fmt.Sprintf("https://%v.jupyter.%s", team, topLevelDomain)
	case gensql.ChartTypeAirflow:
		return fmt.Sprintf("https://%v.airflow.%s", team, topLevelDomain)
	}

	return ""
}

func createAppService(team gensql.TeamsForUserGetRow, chartType gensql.ChartType, topLevelDomain string) *AppService {
	return &AppService{
		App:       string(chartType),
		Ingress:   createIngress(team.Slug, chartType, topLevelDomain),
		Slug:      team.Slug,
		Namespace: k8s.TeamIDToNamespace(team.ID),
	}
}

func (r *Repo) ChartsForTeamGet(ctx context.Context, teamID string) ([]gensql.ChartType, error) {
	return r.querier.ChartsForTeamGet(ctx, teamID)
}

func (r *Repo) ChartDelete(ctx context.Context, teamID string, chartType gensql.ChartType) error {
	return r.querier.ChartDelete(ctx, gensql.ChartDeleteParams{
		TeamID:    teamID,
		ChartType: chartType,
	})
}

func (r *Repo) ServicesForUser(ctx context.Context, email, topLevelDomain string) (UserServices, error) {
	teamsForUser, err := r.querier.TeamsForUserGet(ctx, email)
	if err != nil {
		return UserServices{}, err
	}

	slices.SortFunc(teamsForUser, func(a, b gensql.TeamsForUserGetRow) int {
		if a.ID < b.ID {
			return -1
		} else if a.ID > b.ID {
			return 1
		} else {
			return 0
		}
	})

	var userServices UserServices
	for _, team := range teamsForUser {
		apps, err := r.querier.ChartsForTeamGet(ctx, team.ID)
		if err != nil {
			return UserServices{}, err
		}

		events, err := r.EventLogsForOwnerGet(ctx, team.ID, 3)
		if err != nil {
			return UserServices{}, err
		}

		teamServices := TeamServices{
			TeamID: team.ID,
			Slug:   team.Slug,
			Events: events,
		}

		for _, app := range apps {
			switch app {
			case gensql.ChartTypeJupyterhub:
				teamServices.Jupyterhub = createAppService(team, app, topLevelDomain)
			case gensql.ChartTypeAirflow:
				teamServices.Airflow = createAppService(team, app, topLevelDomain)
			}
		}

		userServices.Services = append(userServices.Services, teamServices)
	}

	var hasUserServices bool
	manager, err := r.querier.UserGoogleSecretManagerGet(ctx, email)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return UserServices{}, err
		}
	} else {
		userServices.UserGSM = &manager
		hasUserServices = true
	}

	if hasUserServices {
		events, err := r.EventLogsForOwnerGet(ctx, email, 3)
		if err != nil {
			return UserServices{}, err
		}

		userServices.UserEvents = events
	}

	return userServices, nil
}

func (r *Repo) TeamValueInsert(ctx context.Context, chartType gensql.ChartType, key, value, teamID string) error {
	return r.querier.TeamValueInsert(ctx, gensql.TeamValueInsertParams{
		Key:       key,
		Value:     value,
		TeamID:    teamID,
		ChartType: chartType,
	})
}

func (r *Repo) HelmChartValuesInsert(ctx context.Context, chartType gensql.ChartType, chartValues map[string]string, teamID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	querier := r.querier.WithTx(tx)
	for key, value := range chartValues {
		err := querier.TeamValueInsert(ctx, gensql.TeamValueInsertParams{
			Key:       key,
			Value:     value,
			TeamID:    teamID,
			ChartType: chartType,
		})
		if err != nil {
			if err := tx.Rollback(); err != nil {
				r.log.WithError(err).Error("rolling back service create transaction - team chart value insert")
			}
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}
