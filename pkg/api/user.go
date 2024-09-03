package api

import (
	"net/http"

	"github.com/navikt/knorten/pkg/api/auth"
	"github.com/navikt/knorten/pkg/api/middlewares"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func (c *client) setupUserRoutes() {
	c.router.GET("/oversikt", func(ctx *gin.Context) {
		session := sessions.Default(ctx)

		user, teams, err := c.getUserAndTeams(ctx)
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

		flashes := session.Flashes()
		err = session.Save()
		if err != nil {
			c.log.WithError(err).Error("problem saving session")
			return
		}

		services, err := c.repo.ServicesForUser(ctx, user.Email, c.topLevelDomain)
		ctx.HTML(http.StatusOK, "oversikt/index", gin.H{
			"errors":                err,
			"flashes":               flashes,
			"user":                  services,
			"gcpProject":            c.gcpProject,
			"gcpZone":               c.gcpZone,
			"upgradePausedStatuses": c.maintenanceExclusionConfig.ActiveExcludePeriodForTeams(teams),
			"loggedIn":              ctx.GetBool(middlewares.LoggedInKey),
			"isAdmin":               ctx.GetBool(middlewares.AdminKey),
		})
	})
}

func (c client) getUserAndTeams(ctx *gin.Context) (*auth.User, []string, error) {
	user, err := getUser(ctx)
	if err != nil {
		return nil, nil, err
	}

	teams, err := c.repo.TeamsForUser(ctx, user.Email)
	if err != nil {
		return nil, nil, err
	}

	return user, teams, nil
}
