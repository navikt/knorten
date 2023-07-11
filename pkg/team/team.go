package team

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/nais/knorten/pkg/auth"
	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/google"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/nais/knorten/pkg/logger"
	"github.com/sirupsen/logrus"
	"github.com/thanhpk/randstr"
	"k8s.io/utils/strings/slices"
)

type Client struct {
	repo         *database.Repo
	googleClient *google.Google
	k8sClient    *k8s.Client
	chartClient  *chart.Client
	azureClient  *auth.Azure
	dryRun       bool
	log          *logrus.Entry
}

func NewClient(repo *database.Repo, googleClient *google.Google, k8sClient *k8s.Client, chartClient *chart.Client, azureClient *auth.Azure, dryRun bool, log *logrus.Entry) *Client {
	return &Client{
		repo:         repo,
		googleClient: googleClient,
		k8sClient:    k8sClient,
		chartClient:  chartClient,
		azureClient:  azureClient,
		dryRun:       dryRun,
		log:          log,
	}
}

type Form struct {
	Slug      string   `form:"team" binding:"required,validTeamName"`
	Owner     string   `form:"owner" binding:"required"`
	Users     []string `form:"users[]" binding:"validEmail"`
	APIAccess string   `form:"apiaccess"`
}

func (c Client) Create(ctx context.Context, form Form, log logger.Logger) {
	log.Infof("Creating team %v", form.Slug)

	team, err := c.repo.TeamGet(ctx, form.Slug)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Errorf("sql error: %v", err)
			return
		}
	}
	if team.Slug == form.Slug {
		log.Fatalf("there already exists a team with name %v", form.Slug)
		return
	}

	teamID := createTeamID(form.Slug)
	users := removeEmptyUsers(form.Users)

	if err := c.ensureUsersExists(users); err != nil {
		log.Fatalf("failed to verify azure user %v: %v", users, err)
		return
	}

	if err := c.googleClient.CreateGCPTeamResources(ctx, form.Slug, teamID, users); err != nil {
		log.Errorf("failed creating gcp team: %v", err)
		return
	}

	if err := c.k8sClient.CreateTeamNamespace(ctx, k8s.NameToNamespace(teamID)); err != nil {
		log.Errorf("failed creating team namespace: %v", err)
		return
	}

	if err := c.k8sClient.CreateTeamServiceAccount(ctx, teamID, k8s.NameToNamespace(teamID)); err != nil {
		log.Errorf("failed creating team SA: %v", err)
		return
	}

	if err := c.repo.TeamCreate(ctx, teamID, form.Slug, form.Owner, users, form.APIAccess == "on"); err != nil {
		log.Errorf("sql error: %v", err)
		return
	}

	log.Infof("Created %v", teamID)
}

func (c Client) Update(ctx *gin.Context) error {
	var form Form
	form.Slug = ctx.Param("team")
	err := ctx.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return err
	}

	team, err := c.repo.TeamGet(ctx, form.Slug)
	if err != nil {
		return err
	}
	users := removeEmptyUsers(form.Users)
	if err := c.ensureUsersExists(users); err != nil {
		return err
	}

	err = c.repo.TeamUpdate(ctx, team.ID, users, form.APIAccess == "on")
	if err != nil {
		return err
	}

	go c.updateExternalResources(ctx, team.Slug)

	apps, err := c.repo.AppsForTeamGet(ctx, team.ID)
	if err != nil {
		return err
	}

	if slices.Contains(apps, string(gensql.ChartTypeJupyterhub)) {
		configValues := chart.JupyterConfigurableValues{}
		if err := c.repo.TeamConfigurableValuesGet(ctx, gensql.ChartTypeJupyterhub, team.ID, &configValues); err != nil {
			return err
		}

		jupyterForm := chart.JupyterForm{
			Slug: form.Slug,
			JupyterValues: chart.JupyterValues{
				JupyterConfigurableValues: configValues,
			},
		}

		err = c.chartClient.Jupyterhub.Update(ctx, jupyterForm)
		if err != nil {
			return err
		}
	}

	if slices.Contains(apps, string(gensql.ChartTypeAirflow)) {
		airflowForm := chart.AirflowForm{
			Slug: form.Slug,
		}

		err = c.chartClient.Airflow.Update(ctx, airflowForm)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c Client) Delete(ctx context.Context, teamSlug string) error {
	team, err := c.repo.TeamGet(ctx, teamSlug)
	if err != nil {
		return err
	}

	apps, err := c.repo.AppsForTeamGet(ctx, team.ID)
	if err != nil {
		return err
	}

	instance, err := c.repo.ComputeInstanceGet(ctx, team.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	if err := c.repo.TeamDelete(ctx, team.ID); err != nil {
		return err
	}

	go c.deleteExternalResources(ctx, team, apps, instance)

	return nil
}

func (c Client) createExternalResources(ctx context.Context, slug, teamID string, users []string) {
	if err := c.googleClient.CreateGCPTeamResources(ctx, slug, teamID, users); err != nil {
		c.log.WithError(err).Error("failed while creating external resources")
		return
	}

	if err := c.k8sClient.CreateTeamNamespace(ctx, k8s.NameToNamespace(teamID)); err != nil {
		c.log.WithError(err).Error("failed while creating external resources")
		return
	}

	if err := c.k8sClient.CreateTeamServiceAccount(ctx, teamID, k8s.NameToNamespace(teamID)); err != nil {
		c.log.WithError(err).Error("failed while creating external resources")
		return
	}
}

func (c Client) updateExternalResources(ctx context.Context, teamSlug string) {
	if err := c.googleClient.Update(ctx, teamSlug); err != nil {
		c.log.WithError(err).Error("failed while updating google resources")
		return
	}
}

func (c Client) deleteExternalResources(ctx context.Context, team gensql.TeamGetRow, apps []string, instance gensql.ComputeInstance) {
	if err := c.googleClient.DeleteGCPTeamResources(ctx, team, instance); err != nil {
		c.log.WithError(err).Error("failed while deleting external resources")
		return
	}

	namespace := k8s.NameToNamespace(team.ID)

	if slices.Contains(apps, string(gensql.ChartTypeJupyterhub)) {
		releaseName := chart.JupyterReleaseName(namespace)
		if err := c.k8sClient.CreateHelmUninstallJob(ctx, team.ID, releaseName); err != nil {
			c.log.WithError(err).Error("create helm uninstall job")
			return
		}
	}

	if slices.Contains(apps, string(gensql.ChartTypeAirflow)) {
		if err := c.googleClient.DeleteCloudSQLInstance(ctx, chart.CreateAirflowDBInstanceName(team.ID)); err != nil {
			c.log.WithError(err).Error("failed while deleting external resources")
			return
		}
	}

	if err := c.k8sClient.DeleteTeamNamespace(ctx, namespace); err != nil {
		c.log.WithError(err).Error("failed while deleting external resources")
		return
	}
}

func (c Client) ensureUsersExists(users []string) error {
	if c.dryRun {
		c.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	for _, u := range users {
		if err := c.azureClient.UserExistsInAzureAD(u); err != nil {
			return err
		}
	}

	return nil
}

func removeEmptyUsers(formUsers []string) []string {
	return slices.Filter(nil, formUsers, func(s string) bool {
		return s != ""
	})
}

func createTeamID(slug string) string {
	if len(slug) > 25 {
		slug = slug[:25]
	}

	return slug + "-" + strings.ToLower(randstr.String(4))
}
