package api

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/nais/knorten/pkg/database/gensql"
)

func (c *client) setupComputeRoutes() {
	c.router.GET("/compute/new", func(ctx *gin.Context) {
		c.htmlResponseWrapper(ctx, http.StatusOK, "compute/new", gin.H{})
	})

	c.router.POST("/compute/new", func(ctx *gin.Context) {
		err := c.createComputeInstance(ctx)
		if err != nil {
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, "/compute/new")
				return
			}
			ctx.Redirect(http.StatusSeeOther, "/compute/new")
			return
		}

		ctx.Redirect(http.StatusSeeOther, "/oversikt")
	})

	c.router.GET("/compute/edit", func(ctx *gin.Context) {
		user, err := getUser(ctx)
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

		c.htmlResponseWrapper(ctx, http.StatusOK, "compute/edit", gin.H{
			"name": "compute-" + getNormalizedNameFromEmail(user.Email),
		})
	})

	c.router.POST("/compute/delete", func(ctx *gin.Context) {
		err := c.deleteComputeInstance(ctx)
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

		ctx.Redirect(http.StatusSeeOther, "/oversikt")
	})
}

func (c *client) deleteComputeInstance(ctx *gin.Context) error {
	user, err := getUser(ctx)
	if err != nil {
		return err
	}

	return c.repo.RegisterDeleteComputeEvent(ctx, user.Email)
}

func (c *client) createComputeInstance(ctx *gin.Context) error {
	user, err := getUser(ctx)
	if err != nil {
		return err
	}

	instance := gensql.ComputeInstance{
		Owner: user.Email,
		Name:  "compute-" + getNormalizedNameFromEmail(user.Email),
	}

	return c.repo.RegisterCreateComputeEvent(ctx, instance.Owner, instance)
}
