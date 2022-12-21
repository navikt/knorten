package team

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/google"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/thanhpk/randstr"
	"k8s.io/utils/strings/slices"
)

type Client struct {
	repo         *database.Repo
	googleClient *google.Google
	k8sClient    *k8s.Client
	chartClient  *chart.Client
}

func NewClient(repo *database.Repo, googleClient *google.Google, k8sClient *k8s.Client, chartClient *chart.Client) *Client {
	return &Client{
		repo:         repo,
		googleClient: googleClient,
		k8sClient:    k8sClient,
		chartClient:  chartClient,
	}
}

type Form struct {
	Slug  string   `form:"team" binding:"required,validTeamName"`
	Users []string `form:"users[]" binding:"required,validEmail"`
}

func (c Client) Create(ctx *gin.Context) error {
	var form Form
	err := ctx.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return err
	}

	_, err = c.repo.TeamGet(ctx, form.Slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fmt.Println("Creating team", form.Slug)
		} else {
			return err
		}
	}

	teamID := createTeamID(form.Slug)

	if err := c.repo.TeamCreate(ctx, teamID, form.Slug, removeEmptyUsers(form.Users)); err != nil {
		return err
	}

	go c.createExternalResources(ctx, form.Slug, teamID, form.Users)

	return nil
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

	err = c.repo.TeamUpdate(ctx, team.ID, removeEmptyUsers(form.Users))
	if err != nil {
		return err
	}

	err = c.googleClient.Update(ctx, team.ID, form.Users)
	if err != nil {
		return err
	}

	apps, err := c.repo.AppsForTeamGet(ctx, team.ID)
	if err != nil {
		return err
	}

	if slices.Contains(apps, string(gensql.ChartTypeJupyterhub)) {
		jupyterForm := chart.JupyterForm{
			Slug:   team.Slug,
			TeamID: team.ID,
			JupyterValues: chart.JupyterValues{
				AdminUsers:   form.Users,
				AllowedUsers: form.Users,
			},
		}
		err = c.chartClient.Jupyterhub.UpdateTeamValuesAndInstallOrUpdate(ctx, jupyterForm)
		if err != nil {
			return err
		}
	}

	if slices.Contains(apps, string(gensql.ChartTypeAirflow)) {
		airflowForm := chart.AirflowForm{
			Slug:   team.Slug,
			TeamID: team.ID,
			Users:  form.Users,
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

	if err := c.repo.TeamDelete(ctx, team.ID); err != nil {
		return err
	}

	go c.deleteExternalResources(ctx, team.ID)

	return nil
}

func (c Client) createExternalResources(ctx *gin.Context, slug, teamID string, users []string) {
	if err := c.googleClient.CreateGCPTeamResources(ctx, slug, teamID, users); err != nil {
		logrus.Error(err)
		return
	}

	if err := c.k8sClient.CreateTeamNamespace(ctx, k8s.NameToNamespace(teamID)); err != nil {
		logrus.Error(err)
		return
	}

	if err := c.k8sClient.CreateTeamServiceAccount(ctx, teamID, k8s.NameToNamespace(teamID)); err != nil {
		logrus.Error(err)
		return
	}
}

func (c Client) deleteExternalResources(ctx context.Context, teamID string) {
	if err := c.googleClient.DeleteGCPTeamResources(ctx, teamID); err != nil {
		logrus.Error(err)
		return
	}

	if err := c.k8sClient.DeleteTeamNamespace(ctx, k8s.NameToNamespace(teamID)); err != nil {
		logrus.Error(err)
		return
	}
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
