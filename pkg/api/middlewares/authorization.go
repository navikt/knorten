package middlewares

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/navikt/knorten/pkg/database"
	"github.com/sirupsen/logrus"
)

const (
	AdminKey    string = "knorten/admin"
	LoggedInKey string = "knorten/logged_in"
)

func SetSessionStatus(log *logrus.Entry, sessionCookie string, repo *database.Repo) gin.HandlerFunc {
	return func(c *gin.Context) {
		status, err := getSession(c, sessionCookie, repo)
		if err != nil {
			log.WithError(err).Error("getting session status")
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}

		c.Set(AdminKey, status.isAdmin)
		c.Set(LoggedInKey, status.isLoggedIn)

		c.Next()
	}
}

type sessionStatus struct {
	isAdmin    bool
	isLoggedIn bool
}

func getSession(c *gin.Context, sessionCookie string, repo *database.Repo) (*sessionStatus, error) {
	cookie, err := c.Cookie(sessionCookie)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return &sessionStatus{
				isAdmin:    false,
				isLoggedIn: false,
			}, nil
		}

		return nil, err
	}

	session, err := repo.SessionGet(c, cookie)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &sessionStatus{
				isAdmin:    false,
				isLoggedIn: false,
			}, nil
		}

		return nil, err
	}

	return &sessionStatus{
		isAdmin:    session.IsAdmin,
		isLoggedIn: len(session.Token) > 0,
	}, nil
}
