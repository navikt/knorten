package middlewares

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/navikt/knorten/pkg/database"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/navikt/knorten/pkg/api/auth"
	"github.com/sirupsen/logrus"
)

const (
	sessionCookie = "knorten_session"
)

func Authenticate(log *logrus.Entry, repo *database.Repo, azureClient *auth.Azure, dryRun bool) gin.HandlerFunc {
	if dryRun {
		return func(ctx *gin.Context) {
			user := &auth.User{
				Name:    "Dum My",
				Email:   "dummy@nav.no",
				Expires: time.Time{},
			}
			ctx.Set("user", user)
			ctx.Next()
		}
	}

	certificates, err := azureClient.FetchCertificates()
	if err != nil {
		log.Fatalf("Fetching signing certificates from IdP: %v", err)
	}

	return func(ctx *gin.Context) {
		sessionToken, err := ctx.Cookie(sessionCookie)
		if err != nil {
			ctx.Redirect(http.StatusSeeOther, "/oauth2/login")
			return
		}

		session, err := repo.SessionGet(ctx, sessionToken)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				ctx.Redirect(http.StatusSeeOther, "/oauth2/login")
				return
			}
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		user, err := azureClient.ValidateUser(certificates, session.AccessToken)
		if err != nil {
			if errors.Is(err, auth.ErrAzureTokenExpired) {
				ctx.Redirect(http.StatusSeeOther, "/oauth2/login")
				return
			}
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized validate user"})
			return
		}

		teamSlug := ctx.Param("slug")
		if teamSlug != "" {
			team, err := repo.TeamBySlugGet(ctx, teamSlug)
			if err != nil {
				log.WithError(err).Errorf("problem checking for authorization %v", user.Email)
				ctx.Redirect(http.StatusSeeOther, "/")
				return
			}

			if !slices.Contains(team.Users, strings.ToLower(user.Email)) {
				sess := sessions.Default(ctx)
				sess.AddFlash(fmt.Sprintf("%v is not authorized", user.Email))
				err = sess.Save()
				if err != nil {
					log.WithError(err).Error("problem saving session")
					ctx.Redirect(http.StatusSeeOther, "/")
					return
				}
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("%v is not part of team %v", user.Email, teamSlug)})
				return
			}
		}

		ctx.Set("user", user)
		ctx.Next()
	}
}
