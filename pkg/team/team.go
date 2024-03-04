package team

import (
	"context"
	"database/sql"
	"errors"

	"github.com/navikt/knorten/pkg/k8s/core"

	"github.com/navikt/knorten/pkg/chart"
	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/database/gensql"
	"github.com/navikt/knorten/pkg/k8s"
	"github.com/navikt/knorten/pkg/logger"
)

type Client struct {
	repo       *database.Repo
	manager    k8s.Manager
	gcpProject string
	gcpRegion  string
	dryRun     bool
}

func NewClient(repo *database.Repo, mngr k8s.Manager, gcpProject, gcpRegion string, dryRun, inCluster bool) (*Client, error) {
	return &Client{
		repo:       repo,
		manager:    mngr,
		gcpProject: gcpProject,
		gcpRegion:  gcpRegion,
		dryRun:     dryRun,
	}, nil
}

func (c Client) Create(ctx context.Context, team gensql.Team, log logger.Logger) bool {
	log.Infof("Creating team %v", team.ID)

	if retry, err := c.create(ctx, team, log); err != nil {
		log.Info("failed creating team")
		return retry
	}

	log.Infof("Successfully created team %v", team.ID)
	return false
}

func (c Client) create(ctx context.Context, team gensql.Team, log logger.Logger) (bool, error) {
	existingTeam, err := c.repo.TeamBySlugGet(ctx, team.Slug)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.WithError(err).Info("failed retrieving team from database")
		return true, err
	}

	if existingTeam.Slug == team.Slug {
		log.Errorf("there already exists a team with name %v", team.Slug)
		return false, err
	}

	if err := c.createGCPTeamResources(ctx, team); err != nil {
		log.WithError(err).Info("failed creating GCP resources")
		return true, err
	}

	namespace := k8s.TeamIDToNamespace(team.ID)
	err = c.manager.ApplyNamespace(ctx, core.NewNamespace(namespace))
	if err != nil {
		log.WithError(err).Info("failed updating k8s namespace")
		return true, err
	}

	if err := c.manager.ApplyServiceAccount(ctx, core.NewServiceAccount(team.ID, namespace)); err != nil {
		log.WithError(err).Info("failed creating k8s service account")
		return true, err
	}

	if err := c.repo.TeamCreate(ctx, team); err != nil {
		log.WithError(err).Info("failed saving team to database")
		return true, err
	}

	return false, nil
}

func (c Client) Update(ctx context.Context, team gensql.Team, log logger.Logger) bool {
	log.Infof("Updating team %v", team.ID)

	if retry, err := c.update(ctx, team, log); err != nil {
		log.Info("failed updating team")
		return retry
	}

	return false
}

func (c Client) update(ctx context.Context, team gensql.Team, log logger.Logger) (bool, error) {
	err := c.repo.TeamUpdate(ctx, team)
	if err != nil {
		log.WithError(err).Info("failed updating team in database")
		return true, err
	}

	namespace := k8s.TeamIDToNamespace(team.ID)
	err = c.manager.ApplyNamespace(ctx, core.NewNamespace(namespace))
	if err != nil {
		log.WithError(err).Info("failed updating k8s namespace")
		return true, err
	}

	if err := c.manager.ApplyServiceAccount(ctx, core.NewServiceAccount(team.ID, namespace)); err != nil {
		log.WithError(err).Info("failed creating k8s service account")
		return true, err
	}

	if err := c.updateGCPTeamResources(ctx, team); err != nil {
		log.WithError(err).Info("failed while updating GCP resources")
		return true, err
	}

	apps, err := c.repo.ChartsForTeamGet(ctx, team.ID)
	if err != nil {
		log.WithError(err).Infof("failed getting apps for team %v", team.ID)
		return true, err
	}

	for _, app := range apps {
		switch app {
		case gensql.ChartTypeJupyterhub:
			log.Info("Trigger update of Jupyter")
			jupyterValues := chart.JupyterConfigurableValues{
				TeamID: team.ID,
			}
			if err := c.repo.RegisterUpdateJupyterEvent(ctx, team.ID, jupyterValues); err != nil {
				log.WithError(err).Info("failed while registering Jupyter update event")
				return true, err
			}
		case gensql.ChartTypeAirflow:
			log.Info("Trigger update of Airflow")
			airflowValues := chart.AirflowConfigurableValues{
				TeamID: team.ID,
			}
			if err := c.repo.RegisterUpdateAirflowEvent(ctx, team.ID, airflowValues); err != nil {
				log.WithError(err).Info("failed while registering Airflow update event")
				return true, err
			}
		}
	}

	log.Infof("Successfully updated team %v", team.Slug)
	return false, nil
}

func (c Client) Delete(ctx context.Context, teamID string, log logger.Logger) bool {
	log.Infof("Deleting team %v", teamID)

	if retry, err := c.delete(ctx, teamID, log); err != nil {
		log.Info("failed updating team")
		return retry
	}

	log.Infof("Successfully deleted team %v", teamID)
	return false
}

func (c Client) delete(ctx context.Context, teamID string, log logger.Logger) (bool, error) {
	team, err := c.repo.TeamGet(ctx, teamID)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		log.WithError(err).Info("failed retrieving team from database")
		return true, err
	}

	if err = c.deleteGCPTeamResources(ctx, team.ID); err != nil {
		log.WithError(err).Info("failed while deleting GCP resources")
		return true, err
	}

	if err = c.manager.DeleteNamespace(ctx, k8s.TeamIDToNamespace(team.ID)); err != nil {
		log.WithError(err).Info("failed while deleting k8s namespace")
		return true, err
	}

	if err = c.repo.TeamDelete(ctx, team.ID); err != nil && errors.Is(err, sql.ErrNoRows) {
		log.WithError(err).Info("failed deleting team from database")
		return true, err
	}

	log.Info("Trigger delete of Airflow")
	// Kun Airflow som har ressurser utenfor clusteret
	if err := c.repo.RegisterDeleteAirflowEvent(ctx, team.ID); err != nil {
		log.WithError(err).Info("failed while registering Airflow delete event")
		return true, err
	}

	log.Infof("Successfully deleted team %v", teamID)
	return false, nil
}
