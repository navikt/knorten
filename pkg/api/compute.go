package api

import (
	"net/http"
	"strconv"

	"github.com/navikt/knorten/pkg/api/middlewares"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/navikt/knorten/pkg/database/gensql"
	"github.com/navikt/knorten/pkg/user"
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
		ctx.HTML(http.StatusOK, "compute/new", gin.H{
			"loggedIn": ctx.GetBool(middlewares.LoggedInKey),
			"isAdmin":  ctx.GetBool(middlewares.AdminKey),
		})
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

		ctx.HTML(http.StatusOK, "compute/edit", gin.H{
			"name":       "compute-" + getNormalizedNameFromEmail(user.Email),
			"gcpZone":    c.gcpZone,
			"gcpProject": c.gcpProject,
			"diskSize":   computeInstance.DiskSize,
			"loggedIn":   ctx.GetBool(middlewares.LoggedInKey),
			"isAdmin":    ctx.GetBool(middlewares.AdminKey),
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

	diskSizeInt, err := strconv.ParseInt(form.DiskSize, 10, 32)
	if err != nil {
		return err
	}
	instance.DiskSize = int32(diskSizeInt)

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

func (c *client) createComputeInstance(ctx *gin.Context) error {
	u, err := getUser(ctx)
	if err != nil {
		return err
	}

	instance := gensql.ComputeInstance{
		Owner:    u.Email,
		Name:     "compute-" + getNormalizedNameFromEmail(u.Email),
		DiskSize: user.DefaultComputeDiskSize,
	}

	return c.repo.RegisterCreateComputeEvent(ctx, instance.Owner, instance)
}
