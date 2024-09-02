package api

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/navikt/knorten/pkg/api/middlewares"
)

func (c *client) setupMaintenanceExclusionRoutes() {
	c.router.GET("/maintenance-exclusion", func(ctx *gin.Context) {
		session := sessions.Default(ctx)

		teams, err := c.getTeamsForUser(ctx)
		if err != nil {
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, "/")
				return
			}

			ctx.Redirect(http.StatusSeeOther, "/")
			return
		}
		ctx.HTML(http.StatusOK, "maintenance-exclusion/list", gin.H{
			"airflowUpgradesPausedPeriods": c.airflowUpgradesPaused.MaintenanceExclusionPeriodsForTeams(teams),
			"loggedIn":                     ctx.GetBool(middlewares.LoggedInKey),
			"isAdmin":                      ctx.GetBool(middlewares.AdminKey),
		})
	})
}

func (c *client) getTeamsForUser(ctx *gin.Context) ([]string, error) {
	user, err := getUser(ctx)
	if err != nil {
		return nil, err
	}

	return c.repo.TeamsForUser(ctx, user.Email)
}
