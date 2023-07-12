package api

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/team"

	"github.com/gin-gonic/gin"
	"github.com/nais/knorten/pkg/admin"
	"github.com/nais/knorten/pkg/auth"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/crypto"
	"github.com/nais/knorten/pkg/google"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/sirupsen/logrus"
)

type client struct {
	azureClient         *auth.Azure
	router              *gin.Engine
	repo                *database.Repo
	log                 *logrus.Entry
	googleClient        *google.Google
	k8sClient           *k8s.Client
	adminClient         *admin.Client
	cryptClient         *crypto.EncrypterDecrypter
	chartClient         *chart.Client
	jupyterChartVersion string
	airflowChartVersion string
	adminGroupMail      string
	dryRun              bool
	adminGroupID        string
}

func New(repo *database.Repo, azureClient *auth.Azure, googleClient *google.Google, k8sClient *k8s.Client, cryptClient *crypto.EncrypterDecrypter, chartClient *chart.Client, teamClient *team.Client, dryRun bool, airflowChartVersion, jupyterChartVersion, sessionKey, adminGroup string, log *logrus.Entry) (*gin.Engine, error) {
	adminClient := admin.New(repo, k8sClient, googleClient, cryptClient, chartClient, airflowChartVersion, jupyterChartVersion)

	router := gin.New()

	router.Use(gin.Recovery())
	router.Use(func(ctx *gin.Context) {
		log.Infof("[GIN] %v %v %v", ctx.Request.Method, ctx.Request.URL.Path, ctx.Writer.Status())
	})

	api := client{
		azureClient:    azureClient,
		router:         router,
		repo:           repo,
		googleClient:   googleClient,
		k8sClient:      k8sClient,
		adminClient:    adminClient,
		cryptClient:    cryptClient,
		log:            log,
		chartClient:    chartClient,
		adminGroupMail: adminGroup,
		dryRun:         dryRun,
	}

	session, err := repo.NewSessionStore(sessionKey)
	if err != nil {
		return nil, err
	}

	api.router.Use(session)
	api.router.Static("/assets", "./assets")
	api.router.LoadHTMLGlob("templates/**/*")
	api.setupUnauthenticatedRoutes()
	api.router.Use(api.authMiddleware())
	api.setupAuthenticatedRoutes()
	api.router.Use(api.adminAuthMiddleware())
	api.setupAdminRoutes()
	err = api.fetchAdminGroupID()
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

func (c *client) setupAPIEndpoints() {
	api := c.router.Group("/api")
	api.POST("/status/:team/:chart", func(ctx *gin.Context) {
		teamID := ctx.Param("team")
		chartType := ctx.Param("chart")

		if err := c.repo.TeamSetPendingUpgrade(ctx, teamID, chartType, false); err != nil {
			c.log.WithError(err).Error("clearing pending upgrade flag in database")
		}

		ctx.JSON(http.StatusOK, map[string]any{"status": "ok"})
	})
}

func (c *client) setupUnauthenticatedRoutes() {
	c.router.GET("/", func(ctx *gin.Context) {
		c.htmlResponseWrapper(ctx, http.StatusOK, "index", gin.H{})
	})

	c.setupAPIEndpoints()
	c.setupAuthRoutes()
}

func (c *client) setupAuthenticatedRoutes() {
	c.setupUserRoutes()
	c.setupTeamRoutes()
	c.setupComputeRoutes()
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

	return cookie != ""
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

func (c *client) fetchAdminGroupID() error {
	if c.dryRun {
		c.log.Infof("NOOP: Running in dry run mode")
		return nil
	}
	id, err := c.azureClient.GetGroupID(c.adminGroupMail)
	if err != nil {
		return fmt.Errorf("retrieve admin group id error: %v", err)
	}
	c.adminGroupID = id
	return nil
}
