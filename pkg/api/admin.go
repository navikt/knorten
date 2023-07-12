package api

import (
	"encoding/gob"
	"fmt"
	"net/http"

	"github.com/nais/knorten/pkg/database/gensql"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type teamInfo struct {
	gensql.Team
	Apps []string
}

func (c *client) setupAdminRoutes() {
	c.router.GET("/admin", func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		flashes := session.Flashes()
		err := session.Save()
		if err != nil {
			c.log.WithError(err).Error("problem saving session")
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err})
			return
		}

		teams, err := c.repo.TeamsGet(ctx)
		if err != nil {
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err})
				return
			}

			ctx.Redirect(http.StatusSeeOther, "/admin")
			return
		}

		teamApps := map[string]teamInfo{}
		for _, team := range teams {
			apps, err := c.repo.AppsForTeamGet(ctx, team.ID)
			if err != nil {
				c.log.WithError(err).Error("problem retrieving apps for teams")
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err})
				return
			}
			teamApps[team.ID] = teamInfo{
				Team: team,
				Apps: apps,
			}
		}

		c.htmlResponseWrapper(ctx, http.StatusOK, "admin/index", gin.H{
			"errors": flashes,
			"teams":  teamApps,
		})
	})

	c.router.GET("/admin/:chart", func(ctx *gin.Context) {
		chartType := getChartType(ctx.Param("chart"))

		values, err := c.repo.GlobalValuesGet(ctx, chartType)
		if err != nil {
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, "/admin")
				return
			}

			ctx.Redirect(http.StatusSeeOther, "/admin")
			return
		}

		session := sessions.Default(ctx)
		flashes := session.Flashes()
		err = session.Save()
		if err != nil {
			c.log.WithError(err).Error("problem saving session")
			ctx.Redirect(http.StatusSeeOther, "/admin")
			return
		}

		c.htmlResponseWrapper(ctx, http.StatusOK, "admin/chart", gin.H{
			"values": values,
			"errors": flashes,
			"chart":  string(chartType),
		})
	})

	c.router.POST("/admin/:chart", func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		chartType := getChartType(ctx.Param("chart"))

		err := ctx.Request.ParseForm()
		if err != nil {
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, "admin")
				return
			}
			ctx.Redirect(http.StatusSeeOther, "admin")
			return
		}

		changedValues, err := c.adminClient.FindGlobalValueChanges(ctx, ctx.Request.PostForm, chartType)
		if err != nil {
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v", chartType))
				return
			}

			ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v", chartType))
			return
		}

		if len(changedValues) == 0 {
			session.AddFlash("Ingen endringer lagret")
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v", chartType))
				return
			}
			ctx.Redirect(http.StatusSeeOther, "/admin")
			return
		}

		gob.Register(changedValues)
		session.AddFlash(changedValues)
		err = session.Save()
		if err != nil {
			c.log.WithError(err).Error("problem saving session")
			ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v", chartType))
			return
		}
		ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v/confirm", chartType))
	})

	c.router.GET("/admin/:chart/confirm", func(ctx *gin.Context) {
		chartType := getChartType(ctx.Param("chart"))
		session := sessions.Default(ctx)
		changedValues := session.Flashes()
		err := session.Save()
		if err != nil {
			c.log.WithError(err).Error("problem saving session")
			ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v", chartType))
			return
		}

		c.htmlResponseWrapper(ctx, http.StatusOK, "admin/confirm", gin.H{
			"changedValues": changedValues,
			"chart":         string(chartType),
		})
	})

	c.router.POST("/admin/:chart/confirm", func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		chartType := getChartType(ctx.Param("chart"))

		err := ctx.Request.ParseForm()
		if err != nil {
			c.log.WithError(err)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v/confirm", chartType))
				return
			}
			ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v/confirm", chartType))
			return
		}

		if err := c.adminClient.UpdateGlobalValues(ctx, ctx.Request.PostForm, chartType); err != nil {
			c.log.WithError(err)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v", chartType))
				return
			}
			ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v", chartType))
			return
		}

		if err != nil {
			c.log.WithError(err)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v/confirm", chartType))
				return
			}
			ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/admin/%v/confirm", chartType))
			return
		}

		ctx.Redirect(http.StatusSeeOther, "/admin")
	})

	c.router.POST("/admin/:chart/sync", func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		chartType := getChartType(ctx.Param("chart"))
		team := ctx.PostForm("team")

		switch chartType {
		case gensql.ChartTypeJupyterhub:
			err := c.chartClient.Jupyterhub.Sync(ctx, team)
			if err != nil {
				c.log.WithError(err).Error("syncing Jupyterhub")
				session.AddFlash(err.Error())
				err = session.Save()
				if err != nil {
					c.log.WithError(err).Error("problem saving session")
				}
			}
		case gensql.ChartTypeAirflow:
			err := c.chartClient.Airflow.Sync(ctx, team)
			if err != nil {
				c.log.WithError(err).Error("syncing Airflow")
				session.AddFlash(err.Error())
				err = session.Save()
				if err != nil {
					c.log.WithError(err).Error("problem saving session")
				}
			}
		}

		ctx.Redirect(http.StatusSeeOther, "/admin")
	})

	c.router.POST("/admin/:chart/sync/all", func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		chartType := getChartType(ctx.Param("chart"))

		if err := c.adminClient.ResyncAll(ctx, chartType); err != nil {
			c.log.WithError(err).Errorf("resyncing all instances of %v", chartType)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
			}
		}

		ctx.Redirect(http.StatusSeeOther, "/admin")
	})

	c.router.POST("/admin/:chart/unlock", func(ctx *gin.Context) {
		session := sessions.Default(ctx)
		chartType := getChartType(ctx.Param("chart"))
		team := ctx.PostForm("team")

		err := c.repo.TeamSetPendingUpgrade(ctx, team, string(chartType), false)
		if err != nil {
			c.log.WithError(err).Errorf("unlocking %v", chartType)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
			}
		}

		ctx.Redirect(http.StatusSeeOther, "/admin")
	})

	c.router.POST("/admin/team/sync/all", func(ctx *gin.Context) {
		session := sessions.Default(ctx)

		if err := c.adminClient.ResyncTeams(ctx); err != nil {
			c.log.WithError(err).Errorf("resyncing all teams")
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
			}
		}

		ctx.Redirect(http.StatusSeeOther, "/admin")
	})
}
