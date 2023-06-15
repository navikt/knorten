package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-playground/validator/v10"

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

func airflowMessageForTag(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%v er et påkrevd felt", fe.Field())
	case "startswith":
		return fmt.Sprintf("%v må starte med 'navikt/'", fe.Field())
	default:
		return fe.Error()
	}
}

func (a *API) setupChartRoutes() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		err := v.RegisterValidation("validAirflowRepo", chart.ValidateAirflowRepo)
		if err != nil {
			a.log.WithError(err).Error("can't register validator")
			return
		}
	}

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		err := v.RegisterValidation("validRepoBranch", chart.ValidateRepoBranch)
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
		default:
			c.JSON(http.StatusBadRequest, map[string]string{
				"status":  strconv.Itoa(http.StatusBadRequest),
				"message": fmt.Sprintf("Chart type %v is not supported", chartType),
			})
			return
		}

		session := sessions.Default(c)
		flashes := session.Flashes()
		err := session.Save()
		if err != nil {
			a.log.WithError(err).Error("problem saving session")
			c.JSON(http.StatusInternalServerError, map[string]string{
				"status":  strconv.Itoa(http.StatusInternalServerError),
				"message": "Internal server error",
			})
			return
		}

		a.htmlResponseWrapper(c, http.StatusOK, fmt.Sprintf("charts/%v", chartType), gin.H{
			"team":   team,
			"form":   form,
			"errors": flashes,
		})
	})

	a.router.POST("/team/:team/:chart/new", func(c *gin.Context) {
		slug := c.Param("team")
		chartType := getChartType(c.Param("chart"))
		var err error

		switch chartType {
		case gensql.ChartTypeJupyterhub:
			err = a.chartClient.Jupyterhub.Create(c, slug)
		case gensql.ChartTypeAirflow:
			err = a.chartClient.Airflow.Create(c, slug)
		default:
			c.JSON(http.StatusBadRequest, map[string]string{
				"status":  strconv.Itoa(http.StatusBadRequest),
				"message": fmt.Sprintf("Chart type %v is not supported", chartType),
			})
			return
		}

		if err != nil {
			session := sessions.Default(c)
			var ve validator.ValidationErrors
			if errors.As(err, &ve) {
				for _, fe := range ve {
					switch chartType {
					case gensql.ChartTypeJupyterhub:
					case gensql.ChartTypeAirflow:
						session.AddFlash(airflowMessageForTag(fe))
					}
				}
			} else {
				session.AddFlash(err.Error())
			}

			err := session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/new", slug, chartType))
				return
			}
			c.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/new", slug, chartType))
			return
		}

		c.Redirect(http.StatusSeeOther, "/oversikt")
	})

	a.router.GET("/team/:team/:chart/edit", func(c *gin.Context) {
		slug := c.Param("team")
		chartType := getChartType(c.Param("chart"))

		team, err := a.repo.TeamGet(c, slug)
		if err != nil {
			session := sessions.Default(c)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, "/oversikt")
				return
			}
			c.Redirect(http.StatusSeeOther, "/oversikt")
			return
		}

		var form any
		switch chartType {
		case gensql.ChartTypeJupyterhub:
			form = &chart.JupyterConfigurableValues{}
		case gensql.ChartTypeAirflow:
			form = &chart.AirflowConfigurableValues{}
		default:
			c.JSON(http.StatusBadRequest, map[string]string{
				"status":  strconv.Itoa(http.StatusBadRequest),
				"message": fmt.Sprintf("Chart type %v is not supported", chartType),
			})
			return
		}

		err = a.repo.TeamConfigurableValuesGet(c, chartType, team.ID, form)
		if err != nil {
			session := sessions.Default(c)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, "/oversikt")
				return
			}
			c.Redirect(http.StatusSeeOther, "/oversikt")
			return
		}

		session := sessions.Default(c)
		flashes := session.Flashes()
		err = session.Save()
		if err != nil {
			a.log.WithError(err).Error("problem saving session")
			return
		}

		a.htmlResponseWrapper(c, http.StatusOK, fmt.Sprintf("charts/%v", chartType), gin.H{
			"team":                  slug,
			"pending_jupyterhub":    team.PendingJupyterUpgrade,
			"pending_airflow":       team.PendingAirflowUpgrade,
			"restrictairflowegress": team.RestrictAirflowEgress,
			"values":                form,
			"errors":                flashes,
		})
	})

	a.router.POST("/team/:team/:chart/edit", func(c *gin.Context) {
		slug := c.Param("team")
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
					c.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/edit", slug, chartType))
					return
				}
				c.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/edit", slug, chartType))
				return
			}
			form.Slug = slug
			err = a.chartClient.Jupyterhub.Update(c, form)
		case gensql.ChartTypeAirflow:
			var form chart.AirflowForm
			err = c.ShouldBindWith(&form, binding.Form)
			if err != nil {
				session := sessions.Default(c)
				var ve validator.ValidationErrors
				if errors.As(err, &ve) {
					for _, fe := range ve {
						session.AddFlash(airflowMessageForTag(fe))
					}
				} else {
					session.AddFlash(err.Error())
				}

				err := session.Save()
				if err != nil {
					a.log.WithError(err).Error("problem saving session")
					c.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/edit", slug, chartType))
					return
				}
				c.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/edit", slug, chartType))
				return
			}
			form.Slug = slug
			err = a.chartClient.Airflow.Update(c, form)
		default:
			c.JSON(http.StatusBadRequest, map[string]string{
				"status":  strconv.Itoa(http.StatusBadRequest),
				"message": fmt.Sprintf("Chart type %v is not supported", chartType),
			})
			return
		}

		if err != nil {
			a.log.WithError(err).Errorf("problem editing chart %v for team %v", chartType, slug)
			session := sessions.Default(c)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/edit", slug, chartType))
				return
			}
			c.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/edit", slug, chartType))
			return
		}
		c.Redirect(http.StatusSeeOther, "/oversikt")
	})

	a.router.POST("/team/:team/:chart/delete", func(c *gin.Context) {
		slug := c.Param("team")
		chartType := getChartType(c.Param("chart"))
		var err error

		switch chartType {
		case gensql.ChartTypeJupyterhub:
			err = a.chartClient.Jupyterhub.Delete(c, slug)
		case gensql.ChartTypeAirflow:
			err = a.chartClient.Airflow.Delete(c, slug)
		default:
			c.JSON(http.StatusBadRequest, map[string]string{
				"status":  strconv.Itoa(http.StatusBadRequest),
				"message": fmt.Sprintf("Chart type %v is not supported", chartType),
			})
			return
		}

		if err != nil {
			a.log.WithError(err).Errorf("problem deleting chart %v for team %v", chartType, slug)
			session := sessions.Default(c)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, "/oversikt")
				return
			}
			c.Redirect(http.StatusSeeOther, "/oversikt")
			return
		}
		c.Redirect(http.StatusSeeOther, "/oversikt")
	})
}
