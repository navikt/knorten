package api

import (
	"github.com/gin-gonic/gin"
	"github.com/navikt/knorten/pkg/api/auth"
	"github.com/navikt/knorten/pkg/database"
	"github.com/sirupsen/logrus"
)

type client struct {
	azureClient *auth.Azure
	router      *gin.Engine
	repo        *database.Repo
	log         *logrus.Entry
	dryRun      bool
	gcpProject  string
	gcpZone     string
}

type Config struct {
	AdminGroupEmail string
	DryRun          bool
	GCPProject      string
	GCPZone         string
}

func New(router *gin.Engine, db *database.Repo, azureClient *auth.Azure, log *logrus.Entry, cfg Config) error {
	router.Use(gin.Recovery())
	router.Use(func(ctx *gin.Context) {
		log.WithField("subsystem", "gin").Infof("%v %v %v", ctx.Request.Method, ctx.Request.URL.Path, ctx.Writer.Status())
	})

	api := client{
		azureClient: azureClient,
		router:      router,
		repo:        db,
		log:         log,
		dryRun:      cfg.DryRun,
		gcpProject:  cfg.GCPProject,
		gcpZone:     cfg.GCPZone,
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
