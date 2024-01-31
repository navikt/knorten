package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/navikt/knorten/pkg/api/middlewares"

	"github.com/gin-gonic/gin"
	"github.com/navikt/knorten/pkg/api/auth"
)

func (c *client) adminAuthMiddleware() gin.HandlerFunc {
	if c.dryRun {
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
	return func(ctx *gin.Context) {
		if !ctx.GetBool(middlewares.AdminKey) {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		}

		ctx.Next()
	}
}

func getUser(ctx *gin.Context) (*auth.User, error) {
	var user *auth.User

	anyUser, exists := ctx.Get("user")
	if !exists {
		return nil, fmt.Errorf("getting user")
	}

	user, ok := anyUser.(*auth.User)
	if !ok {
		return nil, fmt.Errorf("verifying user")
	}

	return user, nil
}

func getNormalizedNameFromEmail(name string) string {
	name = strings.Split(name, "@")[0]
	name = strings.ReplaceAll(name, ".", "-")

	return strings.ToLower(name)
}
