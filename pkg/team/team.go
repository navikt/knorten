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
	"github.com/nais/knorten/pkg/database/crypto"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/google"
	"github.com/nais/knorten/pkg/helm"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/thanhpk/randstr"
	"k8s.io/utils/strings/slices"
)

type Form struct {
	Slug  string   `form:"team" binding:"required,validTeamName"`
	Users []string `form:"users[]" binding:"required,validEmail"`
}

func Create(c *gin.Context, repo *database.Repo, googleClient *google.Google, k8sClient *k8s.Client) error {
	var form Form
	err := c.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return err
	}

	_, err = repo.TeamGet(c, form.Slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fmt.Println("Creating team", form.Slug)
		} else {
			return err
		}
	}

	teamID := createTeamID(form.Slug)

	if err := repo.TeamCreate(c, teamID, form.Slug, removeEmptyUsers(form.Users)); err != nil {
		return err
	}

	go createExternalResources(c, googleClient, k8sClient, form.Slug, teamID, form.Users)

	return nil
}

func Update(c *gin.Context, repo *database.Repo, googleClient *google.Google, helmClient *helm.Client, cryptClient *crypto.EncrypterDecrypter) error {
	var form Form
	form.Slug = c.Param("team")
	err := c.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return err
	}

	team, err := repo.TeamGet(c, form.Slug)
	if err != nil {
		return err
	}

	err = repo.TeamUpdate(c, team.ID, removeEmptyUsers(form.Users))
	if err != nil {
		return err
	}

	err = googleClient.Update(c, team.ID, form.Users)
	if err != nil {
		return err
	}

	apps, err := repo.AppsForTeamGet(c, team.ID)
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
		err = chart.UpdateJupyterTeamValuesAndInstall(c, jupyterForm, repo, helmClient, cryptClient)
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
		err = chart.UpdateAirflow(c, airflowForm, repo, helmClient, cryptClient)
		if err != nil {
			return err
		}
	}

	return nil
}

func Delete(ctx context.Context, teamSlug string, repo *database.Repo, googleClient *google.Google, k8sClient *k8s.Client) error {
	team, err := repo.TeamGet(ctx, teamSlug)
	if err != nil {
		return err
	}

	if err := repo.TeamDelete(ctx, team.ID); err != nil {
		return err
	}

	go deleteExternalResources(ctx, team.ID, googleClient, k8sClient)

	return nil
}

func createExternalResources(c *gin.Context, googleClient *google.Google, k8sClient *k8s.Client, slug, teamID string, users []string) {
	if err := googleClient.CreateGCPTeamResources(c, slug, teamID, users); err != nil {
		logrus.Error(err)
		return
	}

	if err := k8sClient.CreateTeamNamespace(c, k8s.NameToNamespace(teamID)); err != nil {
		logrus.Error(err)
		return
	}

	if err := k8sClient.CreateTeamServiceAccount(c, teamID, k8s.NameToNamespace(teamID)); err != nil {
		logrus.Error(err)
		return
	}
}

func deleteExternalResources(c context.Context, teamID string, googleClient *google.Google, k8sClient *k8s.Client) {
	if err := googleClient.DeleteGCPTeamResources(c, teamID); err != nil {
		logrus.Error(err)
		return
	}

	if err := k8sClient.DeleteTeamNamespace(c, k8s.NameToNamespace(teamID)); err != nil {
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
