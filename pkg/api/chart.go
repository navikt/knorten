package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/database/gensql"
	"net/http"
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
	a.router.GET("/:team/:chart/new", func(c *gin.Context) {
		team := c.Param("team")
		chartType := getChartType(c.Param("chart"))

		var form any

		switch chartType {
		case gensql.ChartTypeJupyterhub:
			form = chart.JupyterForm{}
		case gensql.ChartTypeAirflow:
			form = chart.AirflowForm{}
		}

		c.HTML(http.StatusOK, fmt.Sprintf("charts/%v.tmpl", chartType), gin.H{
			"team": team,
			"form": form,
		})
	})

	a.router.POST("/:team/:chart/new", func(c *gin.Context) {
		team := c.Param("team")
		chartType := getChartType(c.Param("chart"))
		var err error

		switch chartType {
		case gensql.ChartTypeJupyterhub:
			err = chart.CreateJupyterhub(c, team, a.repo, a.helmClient)
		case gensql.ChartTypeAirflow:
			err = chart.CreateAirflow(c, team, a.repo, a.helmClient)
		}

		if err != nil {
			fmt.Println(err)
			// c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Redirect(http.StatusSeeOther, fmt.Sprintf("/chart/%v/new", chartType))
		}
		c.Redirect(http.StatusSeeOther, "/user")
	})

	a.router.GET("/:team/:chart/edit", func(c *gin.Context) {
		team := c.Param("team")
		chartType := getChartType(c.Param("chart"))
		var form any

		switch chartType {
		case gensql.ChartTypeJupyterhub:
			form = &chart.JupyterConfigurableValues{}
		case gensql.ChartTypeAirflow:
			form = &chart.AirflowForm{}
		}

		err := a.repo.TeamConfigurableValuesGet(c, chartType, team, form)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.HTML(http.StatusOK, fmt.Sprintf("charts/%v.tmpl", chartType), gin.H{
			"team":   team,
			"values": form,
		})
	})

	a.router.POST("/:team/:chart/edit", func(c *gin.Context) {
		team := c.Param("team")
		chartType := getChartType(c.Param("chart"))
		var err error

		switch chartType {
		case gensql.ChartTypeJupyterhub:
			err = chart.UpdateJupyterhub(c, team, a.repo, a.helmClient)
		case gensql.ChartTypeAirflow:
			err = chart.CreateAirflow(c, team, a.repo, a.helmClient)
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
