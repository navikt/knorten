package api

import (
	"fmt"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/nais/knorten/pkg/google"
)

func (c *client) setupComputeRoutes() {
	c.router.GET("/team/:team/compute/new", func(ctx *gin.Context) {
		slug := ctx.Param("team")
		machineTypes, err := c.repo.SupportedComputeMachineTypes(ctx)
		if err != nil {
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/compute/new", slug))
				return
			}
			ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/compute/new", slug))
			return
		}

		c.htmlResponseWrapper(ctx, http.StatusOK, "gcp/compute", gin.H{
			"team":          slug,
			"machine_types": machineTypes,
		})
	})

	c.router.POST("/team/:team/compute/new", func(ctx *gin.Context) {
		slug := ctx.Param("team")
		err := c.googleClient.CreateComputeInstance(ctx, slug)
		if err != nil {
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/compute/new", slug))
				return
			}
			ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/compute/new", slug))
			return
		}

		ctx.Redirect(http.StatusSeeOther, "/oversikt")
	})

	c.router.GET("/team/:team/compute/edit", func(ctx *gin.Context) {
		slug := ctx.Param("team")

		machineTypes, err := c.repo.SupportedComputeMachineTypes(ctx)
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

		team, err := c.repo.TeamGet(ctx, slug)
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

		instance, err := c.repo.ComputeInstanceGet(ctx, team.ID)
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

		session := sessions.Default(ctx)
		flashes := session.Flashes()
		err = session.Save()
		if err != nil {
			c.log.WithError(err).Error("problem saving session")
			return
		}

		values := &google.ComputeForm{
			Name:        instance.InstanceName,
			MachineType: string(instance.MachineType),
		}

		c.htmlResponseWrapper(ctx, http.StatusOK, "gcp/compute", gin.H{
			"team":          slug,
			"values":        values,
			"machine_types": machineTypes,
			"errors":        flashes,
		})
	})

	c.router.POST("/team/:team/compute/delete", func(ctx *gin.Context) {
		slug := ctx.Param("team")

		err := c.googleClient.DeleteComputeInstance(ctx, slug)
		if err != nil {
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/compute/new", slug))
				return
			}
			ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/compute/new", slug))
			return
		}

		ctx.Redirect(http.StatusSeeOther, "/oversikt")
	})
}
