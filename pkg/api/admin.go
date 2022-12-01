package api

import (
	"encoding/gob"
	"fmt"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func (a *API) setupAdminRoutes() {
	a.router.GET("/admin", func(c *gin.Context) {
		session := sessions.Default(c)
		flashes := session.Flashes()
		err := session.Save()
		if err != nil {
			a.log.WithError(err).Error("problem saving session")
			return
		}

		teams, err := a.repo.TeamsGet(c)
		if err != nil {
			session := sessions.Default(c)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, "/admin")
				return
			}

			c.Redirect(http.StatusSeeOther, "/admin")
			return
		}

		c.HTML(http.StatusOK, "admin/index", gin.H{
			"errors": flashes,
			"teams":  teams,
		})
	})

	a.router.GET("/admin/:chart", func(c *gin.Context) {
		chartType := getChartType(c.Param("chart"))

		values, err := a.repo.GlobalValuesGet(c, chartType)
		if err != nil {
			session := sessions.Default(c)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, "/admin")
				return
			}

			c.Redirect(http.StatusSeeOther, "/admin")
			return
		}

		session := sessions.Default(c)
		flashes := session.Flashes()
		err = session.Save()
		if err != nil {
			a.log.WithError(err).Error("problem saving session")
			return
		}

		c.HTML(http.StatusOK, "admin/chart", gin.H{
			"values": values,
			"errors": flashes,
			"chart":  string(chartType),
		})
	})

	a.router.POST("/admin/:chart", func(c *gin.Context) {
		session := sessions.Default(c)
		chartType := getChartType(c.Param("chart"))

		err := c.Request.ParseForm()
		if err != nil {
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, "admin")
				return
			}
			c.Redirect(http.StatusSeeOther, "admin")
			return
		}

		changedValues, err := a.adminClient.FindGlobalValueChanges(c, c.Request.PostForm, chartType)
		if err != nil {
			session := sessions.Default(c)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v", chartType))
				return
			}

			c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v", chartType))
			return
		}
		gob.Register(changedValues)

		if len(changedValues) == 0 {
			session.AddFlash("Ingen endringer lagret")
			err = session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v", chartType))
				return
			}
			c.Redirect(http.StatusSeeOther, "/admin")
			return
		}

		session.AddFlash(changedValues)
		err = session.Save()
		if err != nil {
			a.log.WithError(err).Error("problem saving session")
			c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v", chartType))
			return
		}
		c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v/confirm", chartType))
	})

	a.router.GET("/admin/:chart/confirm", func(c *gin.Context) {
		chartType := getChartType(c.Param("chart"))
		session := sessions.Default(c)
		changedValues := session.Flashes()
		err := session.Save()
		if err != nil {
			a.log.WithError(err).Error("problem saving session")
			c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v", chartType))
			return
		}

		c.HTML(http.StatusOK, "admin/confirm", gin.H{
			"changedValues": changedValues,
			"chart":         string(chartType),
		})
	})

	a.router.POST("/admin/:chart/confirm", func(c *gin.Context) {
		session := sessions.Default(c)
		chartType := getChartType(c.Param("chart"))

		err := c.Request.ParseForm()
		if err != nil {
			a.log.WithError(err)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v/confirm", chartType))
				return
			}
			c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v/confirm", chartType))
			return
		}

		if err := a.adminClient.UpdateGlobalValues(c, c.Request.PostForm, chartType); err != nil {
			c.Redirect(http.StatusSeeOther, "/admin")
			return
		}

		if err != nil {
			a.log.WithError(err)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v/confirm", chartType))
				return
			}
			c.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v/confirm", chartType))
			return
		}

		c.Redirect(http.StatusSeeOther, "/admin")
	})
}
