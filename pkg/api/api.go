package api

import (
	"github.com/gin-gonic/gin"
	"github.com/navikt/knorten/pkg/api/auth"
	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/secrets"
	"github.com/sirupsen/logrus"
)

type client struct {
	azureClient    *auth.Azure
	router         *gin.Engine
	repo           *database.Repo
	secretsClient  *secrets.ExternalSecretClient
	log            *logrus.Entry
	dryRun         bool
	gcpProject     string
	gcpRegion      string
	gcpZone        string
	topLevelDomain string
}

func New(router *gin.Engine, db *database.Repo, azureClient *auth.Azure, secretsClient *secrets.ExternalSecretClient, log *logrus.Entry, dryRun bool, project, region, zone, topLevelDomain string) error {
	router.Use(gin.Recovery())
	router.Use(func(ctx *gin.Context) {
		log.WithField("subsystem", "gin").Infof("%v %v %v", ctx.Request.Method, ctx.Request.URL.Path, ctx.Writer.Status())
	})

	api := client{
		azureClient:    azureClient,
		router:         router,
		repo:           db,
		secretsClient:  secretsClient,
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
