package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
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
	"github.com/nais/knorten/pkg/database/gensql"
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

	id, err := createTeamID(form.Slug)
	if err != nil {
		return gensql.Team{}, err
	}

	return gensql.Team{
		ID:    id,
		Slug:  form.Slug,
		Users: form.Users,
		Owner: form.Owner,
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

		user, err := getUser(ctx)
		if err != nil {
			c.log.Error(err)
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "unable to identify logged in user when creating team"})
			return
		}

		c.htmlResponseWrapper(ctx, http.StatusOK, "team/new", gin.H{
			"form":   form,
			"owner":  user.Email,
			"errors": flashes,
		})
	})

	c.router.POST("/team/new", func(ctx *gin.Context) {
		err := c.newTeam(ctx)
		if err != nil {
			c.log.WithError(err).Info("create team")

			session := sessions.Default(ctx)
			var validationErrorse validator.ValidationErrors
			if errors.As(err, &validationErrorse) {
				for _, fieldError := range validationErrorse {
					session.AddFlash(descriptiveMessageForTeamError(fieldError))
				}
			} else {
				session.AddFlash(err.Error())
			}
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

	c.router.GET("/team/:slug/edit", func(ctx *gin.Context) {
		teamSlug := ctx.Param("slug")
		team, err := c.repo.TeamBySlugGet(ctx, teamSlug)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				ctx.JSON(http.StatusNotFound, map[string]string{
					"status":  strconv.Itoa(http.StatusNotFound),
					"message": fmt.Sprintf("team %v does not exist", teamSlug),
				})
				return
			}
			c.log.WithError(err).Errorf("problem getting team %v", teamSlug)
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

	c.router.POST("/team/:slug/edit", func(ctx *gin.Context) {
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

			teamSlug := ctx.Param("slug")
			ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/edit", teamSlug))
			return
		}
		ctx.Redirect(http.StatusSeeOther, "/oversikt")
	})

	c.router.POST("/team/:slug/delete", func(ctx *gin.Context) {
		teamSlug := ctx.Param("slug")
		err := c.deleteTeam(ctx, teamSlug)
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

func descriptiveMessageForTeamError(fieldError validator.FieldError) string {
	switch fieldError.Tag() {
	case "required":
		field := fieldError.Field()
		if field == "Slug" {
			field = "Teamnavn"
		}

		return fmt.Sprintf("%v er et påkrevd felt", field)
	case "validEmail":
		return fmt.Sprintf("'%v' er ikke en godkjent NAV-bruker", fieldError.Value())
	case "validTeamName":
		return "Teamnavn må være med små bokstaver og bindestrek"
	default:
		return fieldError.Error()
	}
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

func createTeamID(slug string) (string, error) {
	if len(slug) > 25 {
		slug = slug[:25]
	}

	randomBytes := make([]byte, 2)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", err
	}

	return slug + "-" + hex.EncodeToString(randomBytes), nil
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

	return c.repo.RegisterCreateTeamEvent(ctx, team)
}

func (c *client) editTeam(ctx *gin.Context) error {
	team, err := formToTeam(ctx)
	if err != nil {
		return err
	}

	existingTeam, err := c.repo.TeamBySlugGet(ctx, team.Slug)
	if err != nil {
		return err
	}

	team.ID = existingTeam.ID
	team.Users = removeEmptyUsers(team.Users)
	return c.repo.RegisterUpdateTeamEvent(ctx, team)
}

func (c *client) ensureUsersExists(users []string) error {
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

func (c *client) deleteTeam(ctx *gin.Context, teamSlug string) error {
	team, err := c.repo.TeamBySlugGet(ctx, teamSlug)
	if err != nil {
		return err
	}

	return c.repo.RegisterDeleteTeamEvent(ctx, team.ID)
}
