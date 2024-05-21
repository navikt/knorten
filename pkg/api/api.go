package api

import (
	"github.com/gin-gonic/gin"
	"github.com/navikt/knorten/pkg/api/auth"
	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/github"
	"github.com/sirupsen/logrus"
)

type client struct {
	azureClient    *auth.Azure
	githubService  *github.Service
	router         *gin.Engine
	repo           *database.Repo
	log            *logrus.Entry
	dryRun         bool
	gcpProject     string
	gcpZone        string
	topLevelDomain string
}

func New(router *gin.Engine, db *database.Repo, azureClient *auth.Azure, log *logrus.Entry, dryRun bool, project, zone, topLevelDomain string, ghService *github.Service) error {
	router.Use(gin.Recovery())
	router.Use(func(ctx *gin.Context) {
		log.WithField("subsystem", "gin").Infof("%v %v %v", ctx.Request.Method, ctx.Request.URL.Path, ctx.Writer.Status())
	})

	api := client{
		azureClient:    azureClient,
		githubService:  ghService,
		router:         router,
		repo:           db,
		log:            log,
		dryRun:         dryRun,
		gcpProject:     project,
		gcpZone:        zone,
		topLevelDomain: topLevelDomain,
	}

	api.setupAuthenticatedRoutes()
	api.router.Use(api.adminAuthMiddleware())
	api.setupAdminRoutes()

	return nil
}

func (c *client) setupAuthenticatedRoutes() {
	c.setupUserRoutes()
	c.setupTeamRoutes()
	c.setupComputeRoutes()
	c.setupSecretRoutes()
	c.setupChartRoutes()
}
