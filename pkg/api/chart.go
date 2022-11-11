package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	chart2 "github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/database/gensql"
	"net/http"
	"strings"
)

func getChartType(chartType string) gensql.ChartType {
	switch chartType {
	case string(gensql.ChartTypeJupyterhub):
		return gensql.ChartTypeJupyterhub
	case string(gensql.ChartTypeAirflow):
		return gensql.ChartTypeAirflow
	default:
		return ""
	}
}

func (a *API) setupChartRoutes() {
	a.router.GET("/chart/:chart/new", func(c *gin.Context) {
		chartType := strings.ToLower(c.Param("chart"))
		var form chart2.JupyterForm
		err := c.ShouldBind(&form)
		fmt.Println(err)
		fmt.Println(form)
		c.HTML(http.StatusOK, fmt.Sprintf("charts/%v.tmpl", chartType), gin.H{
			"form": form,
		})
	})

	a.router.POST("/chart/:chart/new", func(c *gin.Context) {
		chartType := getChartType(c.Param("chart"))
		var err error

		switch chartType {
		case gensql.ChartTypeJupyterhub:
			err = chart2.CreateJupyterhub(c, a.repo, a.helmClient)
		case gensql.ChartTypeAirflow:
			err = chart2.CreateAirflow(c, a.repo, a.helmClient)
		}

		if err != nil {
			fmt.Println(err)
			// c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Redirect(http.StatusSeeOther, fmt.Sprintf("/chart/%v/new", chartType))
		}
		c.Redirect(http.StatusSeeOther, "/user")
	})

	a.router.GET("/chart/:chart/:namespace/edit", func(c *gin.Context) {
		chartType := getChartType(c.Param("chart"))
		namespace := c.Param("namespace")
		var form any

		switch chartType {
		case gensql.ChartTypeJupyterhub:
			form = &chart2.JupyterConfigurableValues{}
		case gensql.ChartTypeAirflow:
			form = &chart2.Airflow{}
		}

		err := a.repo.TeamConfigurableValuesGet(c, chartType, namespace, form)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.HTML(http.StatusOK, fmt.Sprintf("charts/%v.tmpl", chartType), gin.H{
			"values":    form,
			"namespace": namespace,
		})
	})

	a.router.POST("/chart/:chart/:namespace/edit", func(c *gin.Context) {
		chartType := getChartType(c.Param("chart"))
		var err error

		switch chartType {
		case gensql.ChartTypeJupyterhub:
			err = chart2.UpdateJupyterhub(c, a.repo, a.helmClient)
		case gensql.ChartTypeAirflow:
			err = chart2.CreateAirflow(c, a.repo, a.helmClient)
		}

		if err != nil {
			fmt.Println(err)
			// c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Redirect(http.StatusSeeOther, fmt.Sprintf("charts/%v.tmpl", chartType))
			return
		}
		c.Redirect(http.StatusSeeOther, "/user")
	})
}
