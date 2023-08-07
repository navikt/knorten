package team

import (
	"context"
	"database/sql"
	"errors"

	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/nais/knorten/pkg/logger"
	"k8s.io/client-go/kubernetes"
)

type Client struct {
	repo       *database.Repo
	k8sClient  *kubernetes.Clientset
	gcpProject string
	gcpRegion  string
	dryRun     bool
}

func NewClient(repo *database.Repo, gcpProject, gcpRegion string, dryRun, inCluster bool) (*Client, error) {
	k8sClient, err := k8s.CreateClientset(dryRun, inCluster)
	if err != nil {
		return nil, err
	}

	return &Client{
		repo:       repo,
		k8sClient:  k8sClient,
		gcpProject: gcpProject,
		gcpRegion:  gcpRegion,
		dryRun:     dryRun,
	}, nil
}

func (c Client) Create(ctx context.Context, team gensql.Team, log logger.Logger) bool {
	log = log.WithField("team", team.ID)
	log.Infof("Creating team %v", team.ID)

	existingTeam, err := c.repo.TeamGet(ctx, team.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.WithError(err).Error("failed retrieving team from database")
		return true
	}

	if existingTeam.Slug == team.Slug {
		log.Errorf("there already exists a team with name %v", team.Slug)
		return false
	}

	if err := c.createGCPTeamResources(ctx, team); err != nil {
		log.WithError(err).Error("failed creating GCP resources")
		return true
	}

	namespace := k8s.TeamIDToNamespace(team.ID)
	if err := c.createK8sNamespace(ctx, namespace); err != nil {
		log.WithError(err).Error("failed creating team namespace")
		return true
	}

	if err := c.createK8sServiceAccount(ctx, team.ID, namespace); err != nil {
		log.WithError(err).Error("failed creating k8s service account")
		return true
	}

	if err := c.repo.TeamCreate(ctx, team); err != nil {
		log.WithError(err).Error("failed saving team to database")
		return true
	}

	log.Infof("Successfully created team %v", team.ID)
	return false
}

func (c Client) Update(ctx context.Context, team gensql.Team, log logger.Logger) bool {
	log = log.WithField("team", team.ID)
	log.Infof("Updating team %v", team.ID)

	err := c.repo.TeamUpdate(ctx, team)
	if err != nil {
		log.WithError(err).Error("failed updating team in database")
		return true
	}

	if err := c.updateGCPTeamResources(ctx, team); err != nil {
		log.WithError(err).Error("failed while updating GCP resources")
		return true
	}

	log.Info("Trigger update of Jupyter")
	jupyterValues := chart.JupyterConfigurableValues{
		TeamID: team.ID,
	}
	if err := c.repo.RegisterUpdateJupyterEvent(ctx, team.ID, jupyterValues); err != nil {
		log.WithError(err).Error("failed while registering Jupyter update event")
		return true
	}

	log.Info("Trigger update of Airflow")
	airflowValues := chart.AirflowConfigurableValues{
		TeamID: team.ID,
	}
	if err := c.repo.RegisterUpdateAirflowEvent(ctx, team.ID, airflowValues); err != nil {
		log.WithError(err).Error("failed while registering Airflow update event")
		return true
	}

	log.Infof("Successfully updated team %v", team.Slug)
	return false
}

func (c Client) Delete(ctx context.Context, teamID string, log logger.Logger) bool {
	log = log.WithField("team", teamID)
	log.Infof("Deleting team %v", teamID)

	team, err := c.repo.TeamGet(ctx, teamID)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		log.WithError(err).Error("failed retrieving team from database")
		return true
	}

	if err = c.deleteGCPTeamResources(ctx, team.ID); err != nil {
		log.WithError(err).Error("failed while deleting GCP resources")
		return true
	}

	if err = c.deleteK8sNamespace(ctx, k8s.TeamIDToNamespace(team.ID)); err != nil {
		log.WithError(err).Error("failed while deleting k8s namespace")
		return true
	}

	if err = c.repo.TeamDelete(ctx, team.ID); err != nil && errors.Is(err, sql.ErrNoRows) {
		log.WithError(err).Error("failed deleting team from database")
		return true
	}

	log.Info("Trigger delete of Airflow")
	// Kun Airflow som har ressurser utenfor clusteret
	if err := c.repo.RegisterDeleteAirflowEvent(ctx, team.ID); err != nil {
		log.WithError(err).Error("failed while registering Airflow delete event")
		return true
	}

	log.Infof("Successfully deleted team %v", teamID)
	return false
}
