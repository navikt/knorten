package api

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/nais/knorten/pkg/auth"
)

func (c *client) setupUserRoutes() {
	c.router.GET("/oversikt", func(ctx *gin.Context) {
		var user *auth.User
		anyUser, exists := ctx.Get("user")
		if exists {
			user = anyUser.(*auth.User)
		}

		session := sessions.Default(ctx)
		flashes := session.Flashes()
		err := session.Save()
		if err != nil {
			c.log.WithError(err).Error("problem saving session")
			return
		}

		services, err := c.repo.ServicesForUser(ctx, user.Email)
		c.htmlResponseWrapper(ctx, http.StatusOK, "oversikt/index", gin.H{
			"errors":   err,
			"flashes":  flashes,
			"services": services,
		})
	})
}
