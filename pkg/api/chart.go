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

	"github.com/navikt/knorten/pkg/api/middlewares"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/navikt/knorten/pkg/chart"
	"github.com/navikt/knorten/pkg/database/gensql"
)

type jupyterForm struct {
	CPULimit      string   `form:"cpulimit" binding:"validCPUSpec"`
	CPURequest    string   `form:"cpurequest" binding:"validCPUSpec"`
	MemoryLimit   string   `form:"memorylimit" binding:"validMemorySpec"`
	MemoryRequest string   `form:"memoryrequest" binding:"validMemorySpec"`
	ImageName     string   `form:"imagename"`
	ImageTag      string   `form:"imagetag"`
	CullTimeout   string   `form:"culltimeout"`
	PYPIAccess    string   `form:"pypiaccess"`
	Allowlist     []string `form:"allowlist[]"`
}

func (v jupyterForm) MemoryLimitWithoutUnit() string {
	if v.MemoryLimit == "" {
		return ""
	}

	return v.MemoryLimit[:len(v.MemoryLimit)-1]
}

func (v jupyterForm) MemoryRequestWithoutUnit() string {
	if v.MemoryRequest == "" {
		return ""
	}

	return v.MemoryRequest[:len(v.MemoryRequest)-1]
}

type airflowForm struct {
	DagRepo       string `form:"dagrepo" binding:"required,startswith=navikt/,validAirflowRepo"`
	DagRepoBranch string `form:"dagrepobranch" binding:"validRepoBranch"`
	AirflowImage  string `form:"airflowimage" binding:"validAirflowImage"`
	ApiAccess     string `form:"apiaccess"`
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

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		err := v.RegisterValidation("validAirflowImage", chart.ValidateAirflowImage)
		if err != nil {
			c.log.WithError(err).Error("can't register validator")
			return
		}
	}

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		err := v.RegisterValidation("validCPUSpec", chart.ValidateCPUSpec)
		if err != nil {
			c.log.WithError(err).Error("can't register validator")
			return
		}
	}

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		err := v.RegisterValidation("validMemorySpec", chart.ValidateMemorySpec)
		if err != nil {
			c.log.WithError(err).Error("can't register validator")
			return
		}
	}

	c.router.GET("/team/:slug/:chart/new", func(ctx *gin.Context) {
		slug := ctx.Param("slug")
		chartType := getChartType(ctx.Param("chart"))

		var form any
		switch chartType {
		case gensql.ChartTypeJupyterhub:
			form = jupyterForm{}
		case gensql.ChartTypeAirflow:
			form = airflowForm{}
		default:
			ctx.JSON(http.StatusBadRequest, map[string]string{
				"status":  strconv.Itoa(http.StatusBadRequest),
				"message": fmt.Sprintf("Chart type %v is not supported", chartType),
			})
			return
		}

		session := sessions.Default(ctx)
		flashes := session.Flashes()

		team, err := c.repo.TeamBySlugGet(ctx, slug)
		if err != nil {
			err := session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, "/oversikt")
				return
			}
			ctx.Redirect(http.StatusSeeOther, "/oversikt")
			return
		}

		err = session.Save()
		if err != nil {
			c.log.WithField("team", slug).WithField("chart", chartType).WithError(err).Error("problem saving session")
			ctx.JSON(http.StatusInternalServerError, map[string]string{
				"status":  strconv.Itoa(http.StatusInternalServerError),
				"message": "Internal server error",
			})
			return
		}

		ctx.HTML(http.StatusOK, fmt.Sprintf("charts/%v", chartType), gin.H{
			"team":                  slug,
			"form":                  form,
			"errors":                flashes,
			"upgradePausedStatuses": c.maintenanceExclusionConfig.ActiveExcludePeriodForTeams([]string{team.ID}),
			"loggedIn":              ctx.GetBool(middlewares.LoggedInKey),
			"isAdmin":               ctx.GetBool(middlewares.AdminKey),
		})
	})

	c.router.POST("/team/:slug/:chart/new", func(ctx *gin.Context) {
		slug := ctx.Param("slug")
		chartType := getChartType(ctx.Param("chart"))
		log := c.log.WithField("team", slug).WithField("chart", chartType)

		err := c.newChart(ctx, slug, chartType)
		if err != nil {
			session := sessions.Default(ctx)
			var validationErrorse validator.ValidationErrors
			if errors.As(err, &validationErrorse) {
				for _, fieldError := range validationErrorse {
					log.WithError(err).Infof("field error: %v", fieldError)
					session.AddFlash(descriptiveMessageForChartError(fieldError))
				}
			} else {
				log.WithError(err).Info("non-field error")
				session.AddFlash(err.Error())
			}

			err := session.Save()
			if err != nil {
				log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/new", slug, chartType))
				return
			}

			ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/new", slug, chartType))
			return
		}

		ctx.Redirect(http.StatusSeeOther, "/oversikt")
	})

	c.router.GET("/team/:slug/:chart/edit", func(ctx *gin.Context) {
		teamSlug := ctx.Param("slug")
		chartType := getChartType(ctx.Param("chart"))
		log := c.log.WithField("team", teamSlug).WithField("chart", chartType)

		session := sessions.Default(ctx)

		form, teamID, err := c.getEditChart(ctx, teamSlug, chartType)
		if err != nil {
			var validationErrorse validator.ValidationErrors
			if errors.As(err, &validationErrorse) {
				for _, fieldError := range validationErrorse {
					log.WithError(err).Infof("field error: %v", fieldError)
					session.AddFlash(descriptiveMessageForChartError(fieldError))
				}
			} else {
				log.WithError(err).Info("non-field error")
				session.AddFlash(err.Error())
			}

			err := session.Save()
			if err != nil {
				log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, "/oversikt")
				return
			}
			ctx.Redirect(http.StatusSeeOther, "/oversikt")
			return
		}

		flashes := session.Flashes()
		err = session.Save()
		if err != nil {
			log.WithError(err).Error("problem saving session")
			return
		}

		ctx.HTML(http.StatusOK, fmt.Sprintf("charts/%v", chartType), gin.H{
			"team":                  teamSlug,
			"values":                form,
			"errors":                flashes,
			"upgradePausedStatuses": c.maintenanceExclusionConfig.ActiveExcludePeriodForTeams([]string{teamID}),
			"loggedIn":              ctx.GetBool(middlewares.LoggedInKey),
			"isAdmin":               ctx.GetBool(middlewares.AdminKey),
		})
	})

	c.router.POST("/team/:slug/:chart/edit", func(ctx *gin.Context) {
		teamSlug := ctx.Param("slug")
		chartType := getChartType(ctx.Param("chart"))
		log := c.log.WithField("team", teamSlug).WithField("chart", chartType)

		err := c.editChart(ctx, teamSlug, chartType)
		if err != nil {
			session := sessions.Default(ctx)
			var validationErrorse validator.ValidationErrors
			if errors.As(err, &validationErrorse) {
				for _, fieldError := range validationErrorse {
					log.WithError(err).Infof("field error: %v", fieldError)
					session.AddFlash(descriptiveMessageForChartError(fieldError))
				}
			} else {
				log.WithError(err).Info("non-field error")
				session.AddFlash(err.Error())
			}

			err := session.Save()
			if err != nil {
				log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/edit", teamSlug, chartType))
				return
			}

			ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/%v/edit", teamSlug, chartType))
			return
		}

		ctx.Redirect(http.StatusSeeOther, "/oversikt")
	})

	c.router.POST("/team/:slug/:chart/delete", func(ctx *gin.Context) {
		teamSlug := ctx.Param("slug")
		chartTypeString := ctx.Param("chart")
		log := c.log.WithField("team", teamSlug).WithField("chart", chartTypeString)

		err := c.deleteChart(ctx, teamSlug, chartTypeString)
		if err != nil {
			log.WithError(err).Errorf("problem deleting chart %v for team %v", chartTypeString, teamSlug)
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				log.WithError(err).Error("problem saving session")
			}
		}

		ctx.Redirect(http.StatusSeeOther, "/oversikt")
	})
}

func (c *client) getExistingAllowlist(ctx context.Context, teamID string) ([]string, error) {
	extraAnnotations, err := c.repo.TeamValueGet(ctx, "singleuser.extraAnnotations", teamID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []string{}, nil
		}
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
	team, err := c.repo.TeamBySlugGet(ctx, teamSlug)
	if err != nil {
		return err
	}

	switch chartType {
	case gensql.ChartTypeJupyterhub:
		var form jupyterForm
		err := ctx.ShouldBindWith(&form, binding.Form)
		if err != nil {
			return err
		}

		cullTimeout, err := strconv.ParseUint(form.CullTimeout, 10, 64)
		if err != nil {
			return err
		}

		userIdents, err := c.azureClient.ConvertEmailsToIdents(team.Users)
		if err != nil {
			return err
		}

		cpuLimit, err := parseCPU(form.CPULimit)
		if err != nil {
			return err
		}

		cpuRequest, err := parseCPU(form.CPURequest)
		if err != nil {
			return err
		}

		memoryLimit, err := parseMemory(form.MemoryLimit)
		if err != nil {
			return err
		}

		memoryRequest, err := parseMemory(form.MemoryRequest)
		if err != nil {
			return err
		}

		values := chart.JupyterConfigurableValues{
			TeamID:        team.ID,
			UserIdents:    userIdents,
			CPULimit:      cpuLimit,
			CPURequest:    cpuRequest,
			MemoryLimit:   memoryLimit,
			MemoryRequest: memoryRequest,
			ImageName:     form.ImageName,
			ImageTag:      form.ImageTag,
			CullTimeout:   strconv.FormatUint(cullTimeout, 10),
			AllowList:     removeEmptySliceElements(form.Allowlist),
		}

		return c.repo.RegisterCreateJupyterEvent(ctx, team.ID, values)
	case gensql.ChartTypeAirflow:
		var form airflowForm
		err := ctx.ShouldBindWith(&form, binding.Form)
		if err != nil {
			return err
		}

		dagRepoBranch := form.DagRepoBranch
		if dagRepoBranch == "" {
			dagRepoBranch = "main"
		}

		airflowImage := ""
		airflowTag := ""
		if form.AirflowImage != "" {
			imageParts := strings.Split(form.AirflowImage, ":")
			airflowImage = imageParts[0]
			airflowTag = imageParts[1]
		}

		values := chart.AirflowConfigurableValues{
			TeamID:        team.ID,
			DagRepo:       form.DagRepo,
			DagRepoBranch: dagRepoBranch,
			ApiAccess:     form.ApiAccess == "on",
			AirflowImage:  airflowImage,
			AirflowTag:    airflowTag,
		}

		return c.repo.RegisterCreateAirflowEvent(ctx, team.ID, values)
	}

	return fmt.Errorf("chart type %v is not supported", chartType)
}

func (c *client) getEditChart(ctx *gin.Context, teamSlug string, chartType gensql.ChartType) (any, string, error) {
	team, err := c.repo.TeamBySlugGet(ctx, teamSlug)
	if err != nil {
		return nil, "", err
	}

	var chartObjects any
	switch chartType {
	case gensql.ChartTypeJupyterhub:
		chartObjects = &chart.JupyterConfigurableValues{}
	case gensql.ChartTypeAirflow:
		chartObjects = &chart.AirflowConfigurableValues{}
	default:
		return nil, "", fmt.Errorf("chart type %v is not supported", chartType)
	}

	err = c.repo.TeamConfigurableValuesGet(ctx, chartType, team.ID, chartObjects)
	if err != nil {
		return nil, "", err
	}

	var form any
	switch chartType {
	case gensql.ChartTypeJupyterhub:
		jupyterhubValues := chartObjects.(*chart.JupyterConfigurableValues)
		allowlist, err := c.getExistingAllowlist(ctx, team.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, "", err
		}

		pypiAccessTeamValue, err := c.repo.TeamValueGet(ctx, chart.TeamValueKeyPYPIAccess, team.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, "", err
		}

		pypiAccess := "off"
		if pypiAccessTeamValue.Value == "true" {
			pypiAccess = "on"
		}

		form = jupyterForm{
			CPULimit:      jupyterhubValues.CPULimit,
			CPURequest:    jupyterhubValues.CPURequest,
			MemoryLimit:   jupyterhubValues.MemoryLimit,
			MemoryRequest: jupyterhubValues.MemoryRequest,
			ImageName:     jupyterhubValues.ImageName,
			ImageTag:      jupyterhubValues.ImageTag,
			CullTimeout:   jupyterhubValues.CullTimeout,
			PYPIAccess:    pypiAccess,
			Allowlist:     allowlist,
		}
	case gensql.ChartTypeAirflow:
		airflowValues := chartObjects.(*chart.AirflowConfigurableValues)
		apiAccessTeamValue, err := c.repo.TeamValueGet(ctx, chart.TeamValueKeyApiAccess, team.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, "", err
		}

		apiAccess := ""
		if apiAccessTeamValue.Value == "true" {
			apiAccess = "on"
		}

		airflowImage := ""
		if airflowValues.AirflowImage != "" && airflowValues.AirflowTag != "" {
			airflowImage = fmt.Sprintf("%v:%v", airflowValues.AirflowImage, airflowValues.AirflowTag)
		}

		form = airflowForm{
			DagRepo:       airflowValues.DagRepo,
			DagRepoBranch: airflowValues.DagRepoBranch,
			ApiAccess:     apiAccess,
			AirflowImage:  airflowImage,
		}
	}

	return form, team.ID, nil
}

func (c *client) editChart(ctx *gin.Context, teamSlug string, chartType gensql.ChartType) error {
	team, err := c.repo.TeamBySlugGet(ctx, teamSlug)
	if err != nil {
		return err
	}

	switch chartType {
	case gensql.ChartTypeJupyterhub:
		var form jupyterForm
		err := ctx.ShouldBindWith(&form, binding.Form)
		if err != nil {
			return err
		}

		userIdents, err := c.azureClient.ConvertEmailsToIdents(team.Users)
		if err != nil {
			return err
		}

		cpuLimit, err := parseCPU(form.CPULimit)
		if err != nil {
			return err
		}

		cpuRequest, err := parseCPU(form.CPURequest)
		if err != nil {
			return err
		}

		memoryLimit, err := parseMemory(form.MemoryLimit)
		if err != nil {
			return err
		}

		memoryRequest, err := parseMemory(form.MemoryRequest)
		if err != nil {
			return err
		}

		values := chart.JupyterConfigurableValues{
			TeamID:        team.ID,
			UserIdents:    userIdents,
			CPULimit:      cpuLimit,
			CPURequest:    cpuRequest,
			MemoryLimit:   memoryLimit,
			MemoryRequest: memoryRequest,
			ImageName:     form.ImageName,
			ImageTag:      form.ImageTag,
			CullTimeout:   form.CullTimeout,
			AllowList:     removeEmptySliceElements(form.Allowlist),
		}

		return c.repo.RegisterUpdateJupyterEvent(ctx, team.ID, values)
	case gensql.ChartTypeAirflow:
		var form airflowForm
		err := ctx.ShouldBindWith(&form, binding.Form)
		if err != nil {
			return err
		}

		dagRepoBranch := form.DagRepoBranch
		if dagRepoBranch == "" {
			dagRepoBranch = "main"
		}

		airflowImage := ""
		airflowTag := ""
		if form.AirflowImage != "" {
			imageParts := strings.Split(form.AirflowImage, ":")
			airflowImage = imageParts[0]
			airflowTag = imageParts[1]
		}

		values := chart.AirflowConfigurableValues{
			TeamID:        team.ID,
			DagRepo:       form.DagRepo,
			DagRepoBranch: dagRepoBranch,
			ApiAccess:     form.ApiAccess == "on",
			AirflowImage:  airflowImage,
			AirflowTag:    airflowTag,
		}

		return c.repo.RegisterUpdateAirflowEvent(ctx, team.ID, values)
	}

	return fmt.Errorf("chart type %v is not supported", chartType)
}

func (c *client) deleteChart(ctx *gin.Context, teamSlug, chartTypeString string) error {
	team, err := c.repo.TeamBySlugGet(ctx, teamSlug)
	if err != nil {
		return err
	}

	switch getChartType(chartTypeString) {
	case gensql.ChartTypeJupyterhub:
		return c.repo.RegisterDeleteJupyterEvent(ctx, team.ID)
	case gensql.ChartTypeAirflow:
		return c.repo.RegisterDeleteAirflowEvent(ctx, team.ID)
	}

	return fmt.Errorf("chart type %v is not supported", chartTypeString)
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
