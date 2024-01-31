package api

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/navikt/knorten/pkg/database/gensql"
)

func (c *client) setupSecretRoutes() {
	c.router.POST("/secret/new", func(ctx *gin.Context) {
		err := c.createSecret(ctx)
		if err != nil {
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
			}
		}

		ctx.Redirect(http.StatusSeeOther, "/oversikt")
	})

	c.router.POST("/secret/delete", func(ctx *gin.Context) {
		err := c.deleteSecret(ctx)
		if err != nil {
			session := sessions.Default(ctx)
			session.AddFlash(err.Error())
			err := session.Save()
			if err != nil {
				c.log.WithError(err).Error("problem saving session")
			}
		}

		ctx.Redirect(http.StatusSeeOther, "/oversikt")
	})
}

func (c *client) createSecret(ctx *gin.Context) error {
	user, err := getUser(ctx)
	if err != nil {
		return err
	}

	manager := gensql.UserGoogleSecretManager{
		Owner: user.Email,
		Name:  getNormalizedNameFromEmail(user.Email),
	}

	return c.repo.RegisterCreateUserGSMEvent(ctx, manager.Owner, manager)
}

func (c *client) deleteSecret(ctx *gin.Context) error {
	user, err := getUser(ctx)
	if err != nil {
		return err
	}

	return c.repo.RegisterDeleteUserGSMEvent(ctx, user.Email)
}
