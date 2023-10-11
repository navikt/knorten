package api

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/user"
)

type computeForm struct {
	DiskSize string `form:"diskSize" binding:"validDiskSize"`
}

func (c *client) setupComputeRoutes() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		err := v.RegisterValidation("validDiskSize", user.ValidateDiskSize)
		if err != nil {
			c.log.WithError(err).Error("can't register validator")
			return
		}
	}

	c.router.GET("/compute/new", func(ctx *gin.Context) {
		c.htmlResponseWrapper(ctx, http.StatusOK, "compute/new", gin.H{})
	})

	c.router.POST("/compute/new", func(ctx *gin.Context) {
		err := c.createComputeInstance(ctx, database.EventTypeCreateCompute)
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

		computeInstance, err := c.repo.ComputeInstanceGet(ctx, user.Email)
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
			"name":     "compute-" + getNormalizedNameFromEmail(user.Email),
			"diskSize": computeInstance.DiskSize,
		})
	})

	c.router.POST("/compute/edit", func(ctx *gin.Context) {
		if err := c.editCompute(ctx); err != nil {
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

func (c *client) editCompute(ctx *gin.Context) error {
	var form computeForm
	err := ctx.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return err
	}

	user, err := getUser(ctx)
	if err != nil {
		return err
	}

	instance, err := c.repo.ComputeInstanceGet(ctx, user.Email)
	if err != nil {
		return err
	}
	instance.DiskSize = form.DiskSize

	if err := c.repo.RegisterResizeComputeDiskEvent(ctx, user.Email, instance); err != nil {
		return err
	}

	return nil
}

func (c *client) deleteComputeInstance(ctx *gin.Context) error {
	user, err := getUser(ctx)
	if err != nil {
		return err
	}

	return c.repo.RegisterDeleteComputeEvent(ctx, user.Email)
}

func (c *client) createComputeInstance(ctx *gin.Context, event database.EventType) error {
	user, err := getUser(ctx)
	if err != nil {
		return err
	}

	instance := gensql.ComputeInstance{
		Owner:    user.Email,
		Name:     "compute-" + getNormalizedNameFromEmail(user.Email),
		DiskSize: "10",
	}

	return c.repo.RegisterCreateComputeEvent(ctx, instance.Owner, instance)
}
