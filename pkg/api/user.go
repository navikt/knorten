package api

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/nais/knorten/pkg/auth"
	"net/http"
)

func (a *API) setupUserRoutes() {
	a.router.GET("/user", func(c *gin.Context) {
		var user *auth.User
		anyUser, exists := c.Get("user")
		if exists {
			user = anyUser.(*auth.User)
		}

		session := sessions.Default(c)
		flashes := session.Flashes()
		err := session.Save()
		if err != nil {
			a.log.WithError(err).Error("problem saving session")
			return
		}

		services, err := a.repo.ServicesForUser(c, user.Email)
		c.HTML(http.StatusOK, "user/index", gin.H{
			"errors":   err,
			"flashes":  flashes,
			"services": services,
		})
	})
}
