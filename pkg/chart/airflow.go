package chart

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/helm"
	helmApps "github.com/nais/knorten/pkg/helm/applications"
	"github.com/nais/knorten/pkg/reflect"
)

type AirflowForm struct {
	Namespace     string   `form:"namespace"`
	Users         []string `helm:"users"`
	DagRepo       string   `form:"repo" binding:"required" helm:"dags.gitSync.repo"`
	DagRepoBranch string   `form:"branch" helm:"dags.gitSync.branch"`
}

func CreateAirflow(c *gin.Context, teamName string, repo *database.Repo, helmClient *helm.Client) error {
	var form AirflowForm
	err := c.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return err
	}

	form.Namespace = teamName

	if form.DagRepoBranch == "" {
		form.DagRepoBranch = "main"
	}

	chartValues, err := reflect.CreateChartValues(form)
	if err != nil {
		return err
	}

	err = repo.ServiceCreate(c, gensql.ChartTypeJupyterhub, chartValues, form.Namespace)
	if err != nil {
		return err
	}

	application := helmApps.NewAirflow(form.Namespace, repo)
	_, err = application.Chart(c)
	if err != nil {
		return err
	}

	go helmClient.InstallOrUpgrade(string(gensql.ChartTypeAirflow), form.Namespace, application)

	return nil
}

func UpdateAirflow(c *gin.Context, teamName string, repo *database.Repo, helmClient *helm.Client) (AirflowForm, error) {
	fmt.Println("NOOP")
	return AirflowForm{}, nil
}
