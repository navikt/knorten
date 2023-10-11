package api

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func (c *client) setupUserRoutes() {
	// if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
	// 	err := v.RegisterValidation("validDiskSize", user.ValidateDiskSize)
	// 	if err != nil {
	// 		c.log.WithError(err).Error("can't register validator")
	// 		return
	// 	}
	// }

	c.router.GET("/oversikt", func(ctx *gin.Context) {
		session := sessions.Default(ctx)

		user, err := getUser(ctx)
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

		services, err := c.repo.ServicesForUser(ctx, user.Email)
		c.htmlResponseWrapper(ctx, http.StatusOK, "oversikt/index", gin.H{
			"errors":     err,
			"flashes":    flashes,
			"user":       services,
			"gcpProject": c.gcpProject,
			"gcpZone":    c.gcpZone,
		})
	})
}
