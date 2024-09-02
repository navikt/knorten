package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/navikt/knorten/pkg/api/middlewares"
)

func (c *client) setupMaintenanceExclusionRoutes() {
	c.router.GET("/maintenance-exclusion", func(ctx *gin.Context) {
		ctx.HTML(http.StatusOK, "maintenance-exclusion/list", gin.H{
			"maintenanceExclusionPeriods": c.maintenanceExcluded.Periods,
			"loggedIn":                    ctx.GetBool(middlewares.LoggedInKey),
			"isAdmin":                     ctx.GetBool(middlewares.AdminKey),
		})
	})
}
