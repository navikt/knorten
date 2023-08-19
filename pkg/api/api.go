package api

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"text/template"

	"github.com/gin-gonic/gin"
	"github.com/nais/knorten/pkg/api/auth"
	"github.com/nais/knorten/pkg/database"
	"github.com/sirupsen/logrus"
)

type client struct {
	azureClient  *auth.Azure
	router       *gin.Engine
	repo         *database.Repo
	log          *logrus.Entry
	dryRun       bool
	adminGroupID string
	gcpProject   string
	gcpZone      string
}

func New(repo *database.Repo, azureClient *auth.Azure, dryRun bool, sessionKey, adminGroupEmail, gcpProject, gcpZone string, log *logrus.Entry) (*gin.Engine, error) {
	router := gin.New()

	router.Use(gin.Recovery())
	router.Use(func(ctx *gin.Context) {
		log.WithField("subsystem", "gin").Infof("%v %v %v", ctx.Request.Method, ctx.Request.URL.Path, ctx.Writer.Status())
	})

	api := client{
		azureClient: azureClient,
		router:      router,
		repo:        repo,
		log:         log,
		dryRun:      dryRun,
		gcpProject:  gcpProject,
		gcpZone:     gcpZone,
	}

	session, err := repo.NewSessionStore(sessionKey)
	if err != nil {
		return nil, err
	}

	api.router.Use(session)
	api.router.Static("/assets", "./assets")
	api.router.FuncMap = template.FuncMap{
		"toArray": toArray,
	}
	api.router.LoadHTMLGlob("templates/**/*")
	api.setupUnauthenticatedRoutes()
	api.router.Use(api.authMiddleware())
	api.setupAuthenticatedRoutes()
	api.router.Use(api.adminAuthMiddleware())
	api.setupAdminRoutes()
	err = api.fetchAdminGroupID(adminGroupEmail)
	if err != nil {
		return nil, err
	}

	return router, nil
}

func Run(router *gin.Engine, inCluster bool) error {
	if inCluster {
		return router.Run()
	}

	return router.Run("localhost:8080")
}

func (c *client) setupUnauthenticatedRoutes() {
	c.router.GET("/", func(ctx *gin.Context) {
		c.htmlResponseWrapper(ctx, http.StatusOK, "index", gin.H{})
	})

	c.setupAuthRoutes()
}

func (c *client) setupAuthenticatedRoutes() {
	c.setupUserRoutes()
	c.setupTeamRoutes()
	c.setupComputeRoutes()
	c.setupSecretRoutes()
	c.setupChartRoutes()
}

func (c *client) htmlResponseWrapper(ctx *gin.Context, status int, tmplName string, values gin.H) {
	values["loggedIn"] = c.isLoggedIn(ctx)
	values["isAdmin"] = c.isAdmin(ctx)

	ctx.HTML(status, tmplName, values)
}

func (c *client) isLoggedIn(ctx *gin.Context) bool {
	cookie, err := ctx.Cookie(sessionCookie)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return false
		}
		c.log.WithError(err).Error("reading session cookie")
		return false
	}

	session, err := c.repo.SessionGet(ctx, cookie)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false
		}
		c.log.WithError(err).Error("retrieving session from db")
		return false
	}

	return session.Token != ""
}

func (c *client) isAdmin(ctx *gin.Context) bool {
	cookie, err := ctx.Cookie(sessionCookie)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return false
		}
		c.log.WithError(err).Error("reading session cookie")
		return false
	}

	session, err := c.repo.SessionGet(ctx, cookie)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false
		}
		c.log.WithError(err).Error("retrieving session from db")
		return false
	}

	return session.IsAdmin
}

func (c *client) fetchAdminGroupID(adminGroupEmail string) error {
	id, err := c.azureClient.GetGroupID(adminGroupEmail)
	if err != nil {
		return fmt.Errorf("retrieve admin group id error: %v", err)
	}

	c.adminGroupID = id
	return nil
}

func toArray(args ...any) []any {
	return args
}
