package api

import (
	"fmt"
	"github.com/go-playground/validator/v10"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/database/gensql"
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
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		err := v.RegisterValidation("validDagRepo", chart.AirflowValidateDagRepo)
		if err != nil {
			a.log.WithError(err).Error("can't register validator")
			return
		}
	}

	a.router.GET("/team/:team/:chart/new", func(c *gin.Context) {
		team := c.Param("team")
		chartType := getChartType(c.Param("chart"))

		var form any

		switch chartType {
		case gensql.ChartTypeJupyterhub:
			form = chart.JupyterForm{}
		case gensql.ChartTypeAirflow:
			form = chart.AirflowForm{}
		}

		session := sessions.Default(c)
		flashes := session.Flashes()
		err := session.Save()
		if err != nil {
			a.log.WithError(err).Error("problem saving session")
			return
		}

		c.HTML(http.StatusOK, fmt.Sprintf("charts/%v", chartType), gin.H{
			"team":   team,
			"form":   form,
			"errors": flashes,
		})
	})

	a.router.POST("/team/:team/:chart/new", func(c *gin.Context) {
		team := c.Param("team")
		chartType := getChartType(c.Param("chart"))
		var err error

		switch chartType {
		case gensql.ChartTypeJupyterhub:
			err = chart.CreateJupyterhub(c, team, a.repo, a.helmClient)
		case gensql.ChartTypeAirflow:
			err = chart.CreateAirflow(c, team, a.repo, a.googleClient, a.k8sClient, a.helmClient)
		}

		if err != nil {
			session := sessions.Default(c)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/new", team, chartType))
				return
			}
			c.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/new", team, chartType))
			return
		}

		c.Redirect(http.StatusSeeOther, "/user")
	})

	a.router.GET("/team/:team/:chart/edit", func(c *gin.Context) {
		team := c.Param("team")
		chartType := getChartType(c.Param("chart"))
		var form any

		switch chartType {
		case gensql.ChartTypeJupyterhub:
			form = &chart.JupyterConfigurableValues{}
		case gensql.ChartTypeAirflow:
			form = &chart.AirflowForm{}
		}

		err := c.ShouldBindWith(&form, binding.Form)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		err = a.repo.TeamConfigurableValuesGet(c, chartType, team, form)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		session := sessions.Default(c)
		flashes := session.Flashes()
		err = session.Save()
		if err != nil {
			a.log.WithError(err).Error("problem saving session")
			return
		}
		c.HTML(http.StatusOK, fmt.Sprintf("charts/%v", chartType), gin.H{
			"team":   team,
			"values": form,
			"errors": flashes,
		})
	})

	a.router.POST("/team/:team/:chart/edit", func(c *gin.Context) {
		team := c.Param("team")
		chartType := getChartType(c.Param("chart"))
		var err error

		switch chartType {
		case gensql.ChartTypeJupyterhub:
			var form chart.JupyterForm
			err = c.ShouldBindWith(&form, binding.Form)
			if err != nil {
				session := sessions.Default(c)
				session.AddFlash(err.Error())
				err := session.Save()
				if err != nil {
					a.log.WithError(err).Error("problem saving session")
					c.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/edit", team, chartType))
					return
				}
				c.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/edit", team, chartType))
				return
			}
			form.Slug = team
			err = chart.UpdateJupyterhub(c, form, a.repo, a.helmClient)
		case gensql.ChartTypeAirflow:
			var form chart.AirflowForm
			err = c.ShouldBindWith(&form, binding.Form)
			if err != nil {
				session := sessions.Default(c)
				session.AddFlash(err.Error())
				err := session.Save()
				if err != nil {
					a.log.WithError(err).Error("problem saving session")
					c.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/edit", team, chartType))
					return
				}
				c.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/edit", team, chartType))
				return
			}
			form.Slug = team
			err = chart.UpdateAirflow(c, form, a.repo, a.helmClient)
		}

		if err != nil {
			a.log.WithError(err).Errorf("problem editing chart %v for team %v", chartType, team)
			session := sessions.Default(c)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/edit", team, chartType))
				return
			}
			c.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/edit", team, chartType))
			return
		}
		c.Redirect(http.StatusSeeOther, "/user")
	})
}
