package api

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/nais/knorten/pkg/api/auth"
)

func (c *client) setupUserRoutes() {
	c.router.GET("/oversikt", func(ctx *gin.Context) {
		var user *auth.User
		anyUser, exists := ctx.Get("user")
		if !exists {
			ctx.Redirect(http.StatusSeeOther, "/")
			return
		}

		session := sessions.Default(ctx)
		flashes := session.Flashes()
		err := session.Save()
		if err != nil {
			c.log.WithError(err).Error("problem saving session")
			return
		}

		user = anyUser.(*auth.User)
		services, err := c.repo.ServicesForUser(ctx, user.Email)
		c.htmlResponseWrapper(ctx, http.StatusOK, "oversikt/index", gin.H{
			"errors":  err,
			"flashes": flashes,
			"user":    services,
		})
	})
}
