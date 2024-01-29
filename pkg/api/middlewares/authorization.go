package middlewares

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/nais/knorten/pkg/api/auth"

	"github.com/gin-gonic/gin"
	"github.com/nais/knorten/pkg/database"
	"github.com/sirupsen/logrus"
)

const (
	AdminKey    string = "knorten/admin"
	LoggedInKey string = "knorten/logged_in"
)

func SetSessionStatus(log *logrus.Entry, sessionCookie string, repo *database.Repo) gin.HandlerFunc {
	return func(c *gin.Context) {
		session, err := GetSession(c, sessionCookie, repo)
		if err != nil {
			switch {
			case errors.Is(err, http.ErrNoCookie):
				// FIXME: is this really an error, if cookie was never set that seems fine
				log.WithError(err).Error("reading session cookie")
			case errors.Is(err, sql.ErrNoRows):
				log.WithError(err).Error("retrieving session from db")
			}

			c.Set(AdminKey, false)
			c.Set(LoggedInKey, false)

			return
		}

		c.Set(AdminKey, session.IsAdmin)
		c.Set(LoggedInKey, len(session.Token) > 0)

		c.Next()
	}
}

func GetSession(c *gin.Context, sessionCookie string, repo *database.Repo) (*auth.Session, error) {
	cookie, err := c.Cookie(sessionCookie)
	if err != nil {
		return nil, err
	}

	session, err := repo.SessionGet(c, cookie)
	if err != nil {
		return nil, err
	}

	return session, nil
}
