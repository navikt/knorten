package team

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/google"
	"github.com/nais/knorten/pkg/helm"
	"github.com/nais/knorten/pkg/k8s"
	"k8s.io/utils/strings/slices"
)

type Form struct {
	Team  string   `form:"team" binding:"required,validTeam"`
	Users []string `form:"users[]" binding:"required,validEmail"`
}

func Create(c *gin.Context, repo *database.Repo, googleClient *google.Google, k8sClient *k8s.Client) error {
	var form Form
	err := c.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return err
	}

	_, err = repo.TeamGet(c, form.Team)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fmt.Println("Creating team", form.Team)
		} else {
			return err
		}
	}

	if err := repo.TeamCreate(c, form.Team, form.Users); err != nil {
		return err
	}

	go createExternalResources(c, googleClient, k8sClient, &form)

	return nil
}

func Update(c *gin.Context, repo *database.Repo, googleClient *google.Google, helmClient *helm.Client) error {
	var form Form
	form.Team = c.Param("team")
	err := c.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return err
	}

	err = repo.TeamUpdate(c, form.Team, form.Users)
	if err != nil {
		return err
	}

	err = googleClient.Update(c, form.Team, form.Users)
	if err != nil {
		return err
	}

	apps, err := repo.AppsForTeamGet(c, form.Team)
	if err != nil {
		return err
	}

	if slices.Contains(apps, string(gensql.ChartTypeJupyterhub)) {
		jupyterForm := chart.JupyterForm{
			Team: form.Team,
			JupyterValues: chart.JupyterValues{
				AdminUsers:   form.Users,
				AllowedUsers: form.Users,
			},
		}
		err = chart.UpdateJupyterhub(c, jupyterForm, repo, helmClient)
		if err != nil {
			return err
		}
	}

	if slices.Contains(apps, string(gensql.ChartTypeAirflow)) {
		airflowForm := chart.AirflowForm{
			Team:  form.Team,
			Users: form.Users,
		}
		err = chart.UpdateAirflow(c, airflowForm, repo, helmClient)
		if err != nil {
			return err
		}
	}

	return nil
}

func createExternalResources(c *gin.Context, googleClient *google.Google, k8sClient *k8s.Client, form *Form) {
	if err := googleClient.CreateGCPResources(c, form.Team, form.Users); err != nil {
		logrus.Error(err)
		return
	}

	if err := k8sClient.CreateTeamNamespace(c, k8s.NameToNamespace(form.Team)); err != nil {
		logrus.Error(err)
		return
	}

	if err := k8sClient.CreateTeamServiceAccount(c, form.Team, k8s.NameToNamespace(form.Team)); err != nil {
		logrus.Error(err)
		return
	}
}
