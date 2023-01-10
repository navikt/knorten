package api

import (
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

		services, err := a.repo.ServicesForUser(c, user.Email)
		c.HTML(http.StatusOK, "user/index", gin.H{
			"errors":   err,
			"services": services,
		})
	})
}
