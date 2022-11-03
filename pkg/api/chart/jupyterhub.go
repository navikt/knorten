package chart

import (
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
)

type Jupyter struct {
	Namespace string   `form:"namespace" binding:"required"`
	Users     []string `form:"users[]" binding:"required"`
	CPU       string   `form:"cpu"`
	Memory    string   `form:"memory"`
}

func CreateJupyterhub(c *gin.Context, repo *database.Repo, chartType gensql.ChartType) error {
	var form Jupyter
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

	if form.CPU == "" {
		form.CPU = "1"
	}
	err = repo.TeamChartValueInsert(c, "singleuser.cpu.limit", form.CPU, form.Namespace, chartType)
	if err != nil {
		return err
	}
	err = repo.TeamChartValueInsert(c, "singleuser.cpu.guarantee", form.CPU, form.Namespace, chartType)
	if err != nil {
		return err
	}

	if form.Memory == "" {
		form.Memory = "1GB"
	}
	err = repo.TeamChartValueInsert(c, "singleuser.memory.limit", form.Memory, form.Namespace, chartType)
	if err != nil {
		return err
	}
	err = repo.TeamChartValueInsert(c, "singleuser.memory.guarantee", form.Memory, form.Namespace, chartType)
	if err != nil {
		return err
	}

	return nil
}
