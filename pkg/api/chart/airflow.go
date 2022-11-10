package chart

import (
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/reflect"
)

type Airflow struct {
	Namespace     string   `form:"namespace" binding:"required" helm:"namespace"`
	Users         []string `form:"users[]" binding:"required" helm:"users"`
	DagRepo       string   `form:"repo" binding:"required" helm:"dags.gitSync.repo"`
	DagRepoBranch string   `form:"branch" helm:"dags.gitSync.branch"`
}

func CreateAirflow(c *gin.Context, repo *database.Repo, chartType gensql.ChartType) error {
	var form Airflow
	err := c.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return err
	}

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

	return nil
}
