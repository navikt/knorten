package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/database/gensql"
)

const (
	jupyterhubAnnotationKey = "singleuser.extraAnnotations"
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

func (c *client) setupChartRoutes() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		err := v.RegisterValidation("validAirflowRepo", chart.ValidateAirflowRepo)
		if err != nil {
			c.log.WithError(err).Error("can't register validator")
			return
		}
	}

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		err := v.RegisterValidation("validRepoBranch", chart.ValidateRepoBranch)
		if err != nil {
			c.log.WithError(err).Error("can't register validator")
			return
		}
	}

	c.router.GET("/team/:team/:chart/new", func(ctx *gin.Context) {
		team := ctx.Param("team")
		chartType := getChartType(ctx.Param("chart"))

		var form any
		switch chartType {
		case gensql.ChartTypeJupyterhub:
			form = chart.JupyterForm{}
		case gensql.ChartTypeAirflow:
			form = chart.AirflowForm{}
		default:
			ctx.JSON(http.StatusBadRequest, map[string]string{
				"status":  strconv.Itoa(http.StatusBadRequest),
				"message": fmt.Sprintf("Chart type %v is not supported", chartType),
			})
			return
		}

		session := sessions.Default(ctx)
		flashes := session.Flashes()
		err := session.Save()
		if err != nil {
			c.log.WithError(err).Error("problem saving session")
			ctx.JSON(http.StatusInternalServerError, map[string]string{
				"status":  strconv.Itoa(http.StatusInternalServerError),
				"message": "Internal server error",
			})
			return
		}

		c.htmlResponseWrapper(ctx, http.StatusOK, fmt.Sprintf("charts/%v", chartType), gin.H{
			"team":   team,
			"form":   form,
			"errors": flashes,
		})
	})

	c.router.POST("/team/:team/:chart/new", func(ctx *gin.Context) {
		slug := ctx.Param("team")
		chartType := getChartType(ctx.Param("chart"))
		var err error

		switch chartType {
		case gensql.ChartTypeJupyterhub:
			err = c.chartClient.Jupyterhub.Create(ctx, slug)
		case gensql.ChartTypeAirflow:
			err = c.chartClient.Airflow.Create(ctx, slug)
		default:
			ctx.JSON(http.StatusBadRequest, map[string]string{
				"status":  strconv.Itoa(http.StatusBadRequest),
				"message": fmt.Sprintf("Chart type %v is not supported", chartType),
			})
			return
		}

		if err != nil {
			session := sessions.Default(ctx)
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
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/new", slug, chartType))
				return
			}
			ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/new", slug, chartType))
			return
		}

		ctx.Redirect(http.StatusSeeOther, "/oversikt")
	})

	c.router.GET("/team/:team/:chart/edit", func(ctx *gin.Context) {
		slug := ctx.Param("team")
		chartType := getChartType(ctx.Param("chart"))

		team, err := c.repo.TeamGet(ctx, slug)
		if err != nil {
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, "/oversikt")
				return
			}
			ctx.Redirect(http.StatusSeeOther, "/oversikt")
			return
		}

		var form any
		var allowlist []string
		switch chartType {
		case gensql.ChartTypeJupyterhub:
			form = &chart.JupyterConfigurableValues{}
			allowlist, err = c.getExistingAllowlist(ctx, team.ID)
			if err != nil {
				if !errors.Is(err, sql.ErrNoRows) {
					c.log.WithError(err).Error("fetching existing jupyterhub allowlist")
					ctx.Redirect(http.StatusSeeOther, "/oversikt")
				}
			}
		case gensql.ChartTypeAirflow:
			form = &chart.AirflowConfigurableValues{}
		default:
			ctx.JSON(http.StatusBadRequest, map[string]string{
				"status":  strconv.Itoa(http.StatusBadRequest),
				"message": fmt.Sprintf("Chart type %v is not supported", chartType),
			})
			return
		}

		err = c.repo.TeamConfigurableValuesGet(ctx, chartType, team.ID, form)
		if err != nil {
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, "/oversikt")
				return
			}
			ctx.Redirect(http.StatusSeeOther, "/oversikt")
			return
		}

		session := sessions.Default(ctx)
		flashes := session.Flashes()
		err = session.Save()
		if err != nil {
			c.log.WithError(err).Error("problem saving session")
			return
		}

		c.htmlResponseWrapper(ctx, http.StatusOK, fmt.Sprintf("charts/%v", chartType), gin.H{
			"team":                  slug,
			"pending_jupyterhub":    team.PendingJupyterUpgrade,
			"pending_airflow":       team.PendingAirflowUpgrade,
			"restrictairflowegress": team.RestrictAirflowEgress,
			"allowlist":             allowlist,
			"values":                form,
			"errors":                flashes,
		})
	})

	c.router.POST("/team/:team/:chart/edit", func(ctx *gin.Context) {
		slug := ctx.Param("team")
		chartType := getChartType(ctx.Param("chart"))
		var err error

		switch chartType {
		case gensql.ChartTypeJupyterhub:
			var form chart.JupyterForm
			err = ctx.ShouldBindWith(&form, binding.Form)
			if err != nil {
				session := sessions.Default(ctx)
				session.AddFlash(err.Error())
				err := session.Save()
				if err != nil {
					c.log.WithError(err).Error("problem saving session")
					ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/edit", slug, chartType))
					return
				}
				ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/edit", slug, chartType))
				return
			}
			form.Slug = slug
			err = c.chartClient.Jupyterhub.Update(ctx, form)
		case gensql.ChartTypeAirflow:
			var form chart.AirflowForm
			err = ctx.ShouldBindWith(&form, binding.Form)
			if err != nil {
				session := sessions.Default(ctx)
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
					c.log.WithError(err).Error("problem saving session")
					ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/edit", slug, chartType))
					return
				}
				ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/edit", slug, chartType))
				return
			}
			form.Slug = slug
			err = c.chartClient.Airflow.Update(ctx, form)
		default:
			ctx.JSON(http.StatusBadRequest, map[string]string{
				"status":  strconv.Itoa(http.StatusBadRequest),
				"message": fmt.Sprintf("Chart type %v is not supported", chartType),
			})
			return
		}

		if err != nil {
			c.log.WithError(err).Errorf("problem editing chart %v for team %v", chartType, slug)
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/edit", slug, chartType))
				return
			}
			ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/edit", slug, chartType))
			return
		}
		ctx.Redirect(http.StatusSeeOther, "/oversikt")
	})

	c.router.POST("/team/:team/:chart/delete", func(ctx *gin.Context) {
		slug := ctx.Param("team")
		chartType := getChartType(ctx.Param("chart"))
		var err error

		switch chartType {
		case gensql.ChartTypeJupyterhub:
			err = c.chartClient.Jupyterhub.Delete(ctx, slug)
		case gensql.ChartTypeAirflow:
			err = c.chartClient.Airflow.Delete(ctx, slug)
		default:
			ctx.JSON(http.StatusBadRequest, map[string]string{
				"status":  strconv.Itoa(http.StatusBadRequest),
				"message": fmt.Sprintf("Chart type %v is not supported", chartType),
			})
			return
		}

		if err != nil {
			c.log.WithError(err).Errorf("problem deleting chart %v for team %v", chartType, slug)
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, "/oversikt")
				return
			}
			ctx.Redirect(http.StatusSeeOther, "/oversikt")
			return
		}
		ctx.Redirect(http.StatusSeeOther, "/oversikt")
	})
}

func (c *client) getExistingAllowlist(ctx context.Context, teamID string) ([]string, error) {
	extraAnnotations, err := c.repo.TeamValueGet(ctx, jupyterhubAnnotationKey, teamID)
	if err != nil {
		return nil, err
	}

	var annotations map[string]string
	if err := json.Unmarshal([]byte(extraAnnotations.Value), &annotations); err != nil {
		return nil, err
	}

	for k, v := range annotations {
		if k == "allowlist" {
			return strings.Split(v, ","), nil
		}
	}

	return []string{}, nil
}
