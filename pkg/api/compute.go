package api

import (
	"fmt"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func (a *API) setupComputeRoutes() {
	a.router.GET("/team/:team/compute/new", func(c *gin.Context) {
		slug := c.Param("team")
		machineTypes, err := a.repo.SupportedComputeMachineTypes(c)
		if err != nil {
			session := sessions.Default(c)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/compute/new", slug))
				return
			}
			c.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/compute/new", slug))
			return
		}

		a.htmlResponseWrapper(c, http.StatusOK, "gcp/compute", gin.H{
			"team":          slug,
			"machine_types": machineTypes,
		})
	})

	a.router.POST("/team/:team/compute/new", func(c *gin.Context) {
		slug := c.Param("team")
		err := a.googleClient.CreateComputeInstance(c, slug)
		if err != nil {
			session := sessions.Default(c)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/compute/new", slug))
				return
			}
			c.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/compute/new", slug))
			return
		}

		c.Redirect(http.StatusSeeOther, "/oversikt")
	})

	a.router.GET("/team/:team/compute/edit", func(c *gin.Context) {
		slug := c.Param("team")

		machineTypes, err := a.repo.SupportedComputeMachineTypes(c)
		if err != nil {
			session := sessions.Default(c)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, "/oversikt")
				return
			}
			c.Redirect(http.StatusSeeOther, "/oversikt")
			return
		}

		team, err := a.repo.TeamGet(c, slug)
		if err != nil {
			session := sessions.Default(c)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, "/oversikt")
				return
			}
			c.Redirect(http.StatusSeeOther, "/oversikt")
			return
		}

		instance, err := a.repo.ComputeInstanceGet(c, team.ID)
		if err != nil {
			session := sessions.Default(c)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, "/oversikt")
				return
			}
			c.Redirect(http.StatusSeeOther, "/oversikt")
			return
		}

		session := sessions.Default(c)
		flashes := session.Flashes()
		err = session.Save()
		if err != nil {
			a.log.WithError(err).Error("problem saving session")
			return
		}

		a.htmlResponseWrapper(c, http.StatusOK, "gcp/compute", gin.H{
			"team":          slug,
			"current":       string(instance.MachineType),
			"machine_types": machineTypes,
			"errors":        flashes,
		})
	})

	a.router.POST("/team/:team/compute/delete", func(c *gin.Context) {
		slug := c.Param("team")

		err := a.googleClient.DeleteComputeInstance(c, slug)
		if err != nil {
			session := sessions.Default(c)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/compute/new", slug))
				return
			}
			c.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/compute/new", slug))
			return
		}

		c.Redirect(http.StatusSeeOther, "/oversikt")
	})
}
