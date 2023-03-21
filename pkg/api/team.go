package api

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/nais/knorten/pkg/team"
)

func (a *API) setupTeamRoutes() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		err := v.RegisterValidation("validEmail", team.ValidateTeamUsers)
		if err != nil {
			a.log.WithError(err).Error("can't register validator")
			return
		}

		err = v.RegisterValidation("validTeamName", team.ValidateTeamName)
		if err != nil {
			a.log.WithError(err).Error("can't register validator")
			return
		}
	}

	a.router.GET("/team/new", func(c *gin.Context) {
		var form team.Form
		session := sessions.Default(c)
		flashes := session.Flashes()
		err := session.Save()
		if err != nil {
			a.log.WithError(err).Error("problem saving session")
			return
		}

		a.htmlResponseWrapper(c, http.StatusOK, "team/new", gin.H{
			"form":   form,
			"errors": flashes,
		})
	})

	a.router.POST("/team/new", func(c *gin.Context) {
		err := a.teamClient.Create(c)
		if err != nil {
			session := sessions.Default(c)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				return
			}
			c.Redirect(http.StatusSeeOther, "/team/new")
			return
		}
		c.Redirect(http.StatusSeeOther, "/oversikt")
	})

	a.router.GET("/team/:team/edit", func(c *gin.Context) {
		teamName := c.Param("team")
		team, err := a.repo.TeamGet(c, teamName)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				c.JSON(http.StatusNotFound, map[string]string{
					"status":  strconv.Itoa(http.StatusNotFound),
					"message": fmt.Sprintf("team %v does not exist", teamName),
				})
				return
			}
			a.log.WithError(err).Errorf("problem getting team %v", teamName)
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
		a.htmlResponseWrapper(c, http.StatusOK, "team/edit", gin.H{
			"team":   team,
			"errors": flashes,
		})
	})

	a.router.POST("/team/:team/edit", func(c *gin.Context) {
		teamName := c.Param("team")
		err := a.teamClient.Update(c)
		if err != nil {
			session := sessions.Default(c)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				return
			}
			c.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/edit", teamName))
			return
		}
		c.Redirect(http.StatusSeeOther, "/oversikt")
	})

	a.router.POST("/team/:team/delete", func(c *gin.Context) {
		teamName := c.Param("team")

		err := a.teamClient.Delete(c, teamName)
		if err != nil {
			session := sessions.Default(c)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				a.log.WithError(err).Error("problem saving session")
				return
			}
			c.Redirect(http.StatusSeeOther, "/oversikt")
			return
		}
		c.Redirect(http.StatusSeeOther, "/oversikt")
	})
}
