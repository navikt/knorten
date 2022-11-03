package chart

import (
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
)

type Airflow struct {
	Namespace     string   `form:"namespace" binding:"required"`
	Users         []string `form:"users[]" binding:"required"`
	DagRepo       string   `form:"repo" binding:"required"`
	DagRepoBranch string   `form:"branch"`
}

func CreateAirflow(c *gin.Context, repo *database.Repo, chartType gensql.ChartType) error {
	var form Airflow
	err := c.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return err
	}

	for _, user := range form.Users {
		err = repo.UserAppInsert(c, user, form.Namespace, chartType)
		if err != nil {
			return err
		}
	}

	err = repo.TeamChartValueInsert(c, "dags.gitSync.repo", form.DagRepo, form.Namespace, chartType)
	if err != nil {
		return err
	}

	if form.DagRepoBranch == "" {
		form.DagRepoBranch = "main"
	}
	err = repo.TeamChartValueInsert(c, "dags.gitSync.branch", form.DagRepoBranch, form.Namespace, chartType)
	if err != nil {
		return err
	}

	return nil
}
