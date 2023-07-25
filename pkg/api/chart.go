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

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/database/gensql"
)

const (
	jupyterhubAnnotationKey = "singleuser.extraAnnotations"
)

type JupyterForm struct {
	CPU         string   `form:"cpu"`
	Memory      string   `form:"memory"`
	ImageName   string   `form:"imagename"`
	ImageTag    string   `form:"imagetag"`
	CullTimeout string   `form:"culltimeout"`
	Allowlist   []string `form:"allowlist[]"`
}

func (v JupyterForm) MemoryWithoutUnit() string {
	if v.Memory == "" {
		return ""
	}

	return v.Memory[:len(v.Memory)-1]
}

type AirflowForm struct {
	Slug           string
	DagRepo        string `form:"dagrepo" binding:"required,startswith=navikt/,validAirflowRepo"`
	DagRepoBranch  string `form:"dagrepobranch" binding:"validRepoBranch"`
	ApiAccess      string `form:"apiaccess"`
	RestrictEgress string `form:"restrictegress"`
}

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

func descriptiveMessageForChartError(fieldError validator.FieldError) string {
	switch fieldError.Tag() {
	case "required":
		return fmt.Sprintf("%v er et påkrevd felt", fieldError.Field())
	case "startswith":
		return fmt.Sprintf("%v må starte med 'navikt/'", fieldError.Field())
	default:
		return fieldError.Error()
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
			form = JupyterForm{}
		case gensql.ChartTypeAirflow:
			form = AirflowForm{}
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
		team := ctx.Param("team")
		chartType := getChartType(ctx.Param("chart"))

		err := c.newChart(ctx, team, chartType)
		if err != nil {
			session := sessions.Default(ctx)
			var validationErrorse validator.ValidationErrors
			if errors.As(err, &validationErrorse) {
				for _, fieldError := range validationErrorse {
					session.AddFlash(descriptiveMessageForChartError(fieldError))
				}
			} else {
				session.AddFlash(err.Error())
			}
			err := session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/new", team, chartType))
				return
			}

			ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/new", team, chartType))
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

		err := c.editChart(ctx, slug, chartType)
		if err != nil {
			session := sessions.Default(ctx)
			var validationErrorse validator.ValidationErrors
			if errors.As(err, &validationErrorse) {
				for _, fieldError := range validationErrorse {
					session.AddFlash(descriptiveMessageForChartError(fieldError))
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
		ctx.Redirect(http.StatusSeeOther, "/oversikt")
	})

	c.router.POST("/team/:team/:chart/delete", func(ctx *gin.Context) {
		team := ctx.Param("team")
		chartType := getChartType(ctx.Param("chart"))

		err := c.deleteChart(ctx, team, chartType)

		if err != nil {
			c.log.WithError(err).Errorf("problem deleting chart %v for team %v", chartType, team)
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

func (c *client) newChart(ctx *gin.Context, teamSlug string, chartType gensql.ChartType) error {
	switch chartType {
	case gensql.ChartTypeJupyterhub:
		var form JupyterForm
		err := ctx.ShouldBindWith(&form, binding.Form)
		if err != nil {
			return err
		}

		cullTimeout, err := strconv.ParseUint(form.CullTimeout, 10, 64)
		if err != nil {
			return err
		}

		team, err := c.repo.TeamGet(ctx, teamSlug)
		if err != nil {
			return err
		}

		userIdents, err := c.convertEmailsToIdents(team.Users)
		if err != nil {
			return err
		}

		values := chart.JupyterConfigurableValues{
			Slug:        teamSlug,
			UserIdents:  userIdents,
			CPU:         form.CPU,
			Memory:      form.Memory,
			ImageName:   form.ImageName,
			ImageTag:    form.ImageTag,
			CullTimeout: strconv.FormatUint(cullTimeout, 10),
		}

		return c.repo.RegisterCreateJupyterEvent(ctx, values)
	case gensql.ChartTypeAirflow:
		var form AirflowForm
		err := ctx.ShouldBindWith(&form, binding.Form)
		if err != nil {
			return err
		}

		apiAccess, err := strconv.ParseBool(form.ApiAccess)
		if err != nil {
			return err
		}

		restrictEgress, err := strconv.ParseBool(form.RestrictEgress)
		if err != nil {
			return err
		}

		dagRepoBranch := form.DagRepoBranch
		if dagRepoBranch == "" {
			dagRepoBranch = "main"
		}

		values := chart.AirflowConfigurableValues{
			Slug:           teamSlug,
			DagRepo:        form.DagRepo,
			DagRepoBranch:  dagRepoBranch,
			ApiAccess:      apiAccess,
			RestrictEgress: restrictEgress,
		}

		return c.repo.RegisterCreateAirflowEvent(ctx, values)
	}

	return fmt.Errorf("chart type %v is not supported", chartType)
}

func (c *client) editChart(ctx *gin.Context, teamSlug string, chartType gensql.ChartType) error {
	switch chartType {
	case gensql.ChartTypeJupyterhub:
		var form JupyterForm
		err := ctx.ShouldBindWith(&form, binding.Form)
		if err != nil {
			return err
		}

		team, err := c.repo.TeamGet(ctx, teamSlug)
		if err != nil {
			return err
		}

		userIdents, err := c.convertEmailsToIdents(team.Users)
		if err != nil {
			return err
		}

		cpu, err := parseCPU(form.CPU)
		if err != nil {
			return err
		}

		memory, err := parseMemory(form.Memory)
		if err != nil {
			return err
		}

		values := chart.JupyterConfigurableValues{
			Slug:        teamSlug,
			UserIdents:  userIdents,
			CPU:         cpu,
			Memory:      memory,
			ImageName:   form.ImageName,
			ImageTag:    form.ImageTag,
			CullTimeout: form.CullTimeout,
		}

		return c.repo.RegisterUpdateJupyterEvent(ctx, values)
	case gensql.ChartTypeAirflow:
		var form AirflowForm
		err := ctx.ShouldBindWith(&form, binding.Form)
		if err != nil {
			return err
		}

		apiAccess, err := strconv.ParseBool(form.ApiAccess)
		if err != nil {
			return err
		}

		restrictEgress, err := strconv.ParseBool(form.RestrictEgress)
		if err != nil {
			return err
		}

		dagRepoBranch := form.DagRepoBranch
		if dagRepoBranch == "" {
			dagRepoBranch = "main"
		}

		values := chart.AirflowConfigurableValues{
			Slug:           teamSlug,
			DagRepo:        form.DagRepo,
			DagRepoBranch:  dagRepoBranch,
			ApiAccess:      apiAccess,
			RestrictEgress: restrictEgress,
		}

		return c.repo.RegisterUpdateAirflowEvent(ctx, values)
	}

	return fmt.Errorf("chart type %v is not supported", chartType)
}

func (c *client) deleteChart(ctx *gin.Context, team string, chartType gensql.ChartType) error {
	switch chartType {
	case gensql.ChartTypeJupyterhub:
		return c.repo.RegisterDeleteJupyterEvent(ctx, team)
	case gensql.ChartTypeAirflow:
		return c.repo.RegisterDeleteAirflowEvent(ctx, team)
	}

	return fmt.Errorf("chart type %v is not supported", chartType)
}

func parseCPU(cpu string) (string, error) {
	floatVal, err := strconv.ParseFloat(cpu, 64)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%.1f", floatVal), nil
}

func parseMemory(memory string) (string, error) {
	if strings.HasSuffix(memory, "G") {
		return memory, nil
	}
	_, err := strconv.ParseFloat(memory, 64)
	if err != nil {
		return "", err
	}
	return memory + "G", nil
}
