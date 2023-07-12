package api

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/nais/knorten/pkg/auth"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/thanhpk/randstr"
	"k8s.io/utils/strings/slices"
)

type teamForm struct {
	Slug      string   `form:"team" binding:"required,validTeamName"`
	Owner     string   `form:"owner" binding:"required"`
	Users     []string `form:"users[]" binding:"validEmail"`
	APIAccess string   `form:"apiaccess"`
}

func formToTeam(ctx *gin.Context) (gensql.Team, error) {
	var form teamForm
	err := ctx.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return gensql.Team{}, err
	}

	return gensql.Team{
		Slug:      ctx.Param("team"),
		Users:     form.Users,
		ApiAccess: form.APIAccess == "on",
		Owner:     form.Owner,
	}, nil
}

func (c *client) setupTeamRoutes() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		err := v.RegisterValidation("validEmail", ValidateTeamUsers)
		if err != nil {
			c.log.WithError(err).Error("can't register validator")
			return
		}

		err = v.RegisterValidation("validTeamName", ValidateTeamName)
		if err != nil {
			c.log.WithError(err).Error("can't register validator")
			return
		}
	}

	c.router.GET("/team/new", func(ctx *gin.Context) {
		var form teamForm
		session := sessions.Default(ctx)
		flashes := session.Flashes()
		err := session.Save()
		if err != nil {
			c.log.WithError(err).Error("problem saving session")
			return
		}

		user, exists := ctx.Get("user")
		if !exists {
			c.log.Errorf("unable to identify logged in user when creating team")
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "unable to identify logged in user when creating team"})
			return
		}
		owner, ok := user.(*auth.User)
		if !ok {
			c.log.Errorf("unable to identify logged in user when creating team, user object %v", user)
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "unable to identify logged in user when creating team"})
			return
		}

		c.htmlResponseWrapper(ctx, http.StatusOK, "team/new", gin.H{
			"form":   form,
			"owner":  owner.Email,
			"errors": flashes,
		})
	})

	c.router.POST("/team/new", func(ctx *gin.Context) {
		err := c.newTeam(ctx)
		if err != nil {
			c.log.WithError(err).Info("create team")
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err = session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				return
			}
			ctx.Redirect(http.StatusSeeOther, "/team/new")
			return
		}
		ctx.Redirect(http.StatusSeeOther, "/oversikt")
	})

	c.router.GET("/team/:team/edit", func(ctx *gin.Context) {
		teamName := ctx.Param("team")
		team, err := c.repo.TeamGet(ctx, teamName)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				ctx.JSON(http.StatusNotFound, map[string]string{
					"status":  strconv.Itoa(http.StatusNotFound),
					"message": fmt.Sprintf("team %v does not exist", teamName),
				})
				return
			}
			c.log.WithError(err).Errorf("problem getting team %v", teamName)
			ctx.Redirect(http.StatusSeeOther, "/oversikt")
			return
		}

		// Avoid duplicating owner as a user in edit form
		team.Users = slices.Filter(nil, team.Users, func(s string) bool {
			return s != team.Owner
		})

		session := sessions.Default(ctx)
		flashes := session.Flashes()
		err = session.Save()
		if err != nil {
			c.log.WithError(err).Error("problem saving session")
			return
		}
		c.htmlResponseWrapper(ctx, http.StatusOK, "team/edit", gin.H{
			"team":   team,
			"errors": flashes,
		})
	})

	c.router.POST("/team/:team/edit", func(ctx *gin.Context) {
		err := c.editTeam(ctx)
		if err != nil {
			c.log.WithError(err).Info("update team")
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				return
			}
			teamName := ctx.Param("team")
			ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/edit", teamName))
			return
		}
		ctx.Redirect(http.StatusSeeOther, "/oversikt")
	})

	c.router.POST("/team/:team/delete", func(ctx *gin.Context) {
		teamName := ctx.Param("team")
		err := c.repo.RegisterDeleteTeamEvent(ctx, teamName)
		if err != nil {
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
				return
			}
			ctx.Redirect(http.StatusSeeOther, "/oversikt")
			return
		}
		ctx.Redirect(http.StatusSeeOther, "/oversikt")
	})
}

var ValidateTeamName validator.Func = func(fl validator.FieldLevel) bool {
	teamSlug := fl.Field().Interface().(string)

	r, _ := regexp.Compile("^[a-z-]+$")
	return r.MatchString(teamSlug)
}

var ValidateTeamUsers validator.Func = func(fl validator.FieldLevel) bool {
	users, ok := fl.Field().Interface().([]string)
	if !ok {
		return false
	}

	for _, user := range users {
		if user == "" {
			continue
		}
		_, err := mail.ParseAddress(user)
		if err != nil {
			return false
		}
		if !strings.HasSuffix(strings.ToLower(user), "nav.no") {
			return false
		}
	}

	return true
}

func createTeamID(slug string) string {
	if len(slug) > 25 {
		slug = slug[:25]
	}

	return slug + "-" + strings.ToLower(randstr.String(4))
}

func (c *client) newTeam(ctx *gin.Context) error {
	team, err := formToTeam(ctx)
	if err != nil {
		return err
	}

	team.Users = removeEmptyUsers(team.Users)
	err = c.ensureUsersExists(team.Users)
	if err != nil {
		return err
	}

	team.ID = createTeamID(team.Slug)
	return c.repo.RegisterCreateTeamEvent(ctx, team)
}

func (c *client) editTeam(ctx *gin.Context) error {
	team, err := formToTeam(ctx)
	if err != nil {
		return err
	}

	team.Users = removeEmptyUsers(team.Users)
	existingTeam, err := c.repo.TeamGet(ctx, team.Slug)
	if err != nil {
		return err
	}

	team.ID = existingTeam.ID
	return c.repo.RegisterUpdateTeamEvent(ctx, team)
}

func (c *client) ensureUsersExists(users []string) error {
	if c.dryRun {
		c.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	for _, u := range users {
		if err := c.azureClient.UserExistsInAzureAD(u); err != nil {
			return err
		}
	}

	return nil
}

func removeEmptyUsers(formUsers []string) []string {
	return slices.Filter(nil, formUsers, func(s string) bool {
		return s != ""
	})
}
