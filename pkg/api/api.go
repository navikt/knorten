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

type API struct {
	azureClient         *auth.Azure
	router              *gin.Engine
	repo                *database.Repo
	log                 *logrus.Entry
	googleClient        *google.Google
	k8sClient           *k8s.Client
	adminClient         *admin.Client
	cryptClient         *crypto.EncrypterDecrypter
	chartClient         *chart.Client
	teamClient          *team.Client
	jupyterChartVersion string
	airflowChartVersion string
	adminGroupMail      string
	dryRun              bool
	adminGroupID        string
}

func New(repo *database.Repo, azureClient *auth.Azure, googleClient *google.Google, k8sClient *k8s.Client, cryptClient *crypto.EncrypterDecrypter, dryRun bool, airflowChartVersion, jupyterChartVersion, sessionKey, adminGroupMail string, log *logrus.Entry) (*gin.Engine, error) {
	adminClient := admin.New(repo, k8sClient, cryptClient, airflowChartVersion, jupyterChartVersion)
	chartClient, err := chart.New(repo, googleClient, k8sClient, cryptClient, airflowChartVersion, jupyterChartVersion, log)
	if err != nil {
		return nil, err
	}

	router := gin.New()

	router.Use(gin.Recovery())
	router.Use(func(ctx *gin.Context) {
		log.Infof("[GIN] %v %v %v", ctx.Request.Method, ctx.Request.URL.Path, ctx.Writer.Status())
	})

	api := API{
		azureClient:    azureClient,
		router:         router,
		repo:           repo,
		googleClient:   googleClient,
		k8sClient:      k8sClient,
		adminClient:    adminClient,
		cryptClient:    cryptClient,
		log:            log,
		chartClient:    chartClient,
		adminGroupMail: adminGroupMail,
		dryRun:         dryRun,
	}

	api.teamClient = team.NewClient(repo, googleClient, k8sClient, api.chartClient, azureClient, dryRun, log.WithField("subsystem", "teamClient"))

	session, err := repo.NewSessionStore(sessionKey)
	if err != nil {
		return nil, err
	}

	api.router.Use(session)
	api.router.Static("/assets", "./assets")
	api.router.LoadHTMLGlob("templates/**/*")
	api.setupUnauthenticatedRoutes()
	api.router.Use(api.authMiddleware([]string{}))
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

func (a *API) setupAPIEndpoints() {
	api := a.router.Group("/api")
	api.POST("/status/:team/:chart", func(c *gin.Context) {
		teamID := c.Param("team")
		chartType := c.Param("chart")

		if err := a.repo.TeamSetPendingUpgrade(c, teamID, chartType, false); err != nil {
			a.log.WithError(err).Error("clearing pending upgrade flag in database")
		}

		c.JSON(http.StatusOK, map[string]any{"status": "ok"})
	})
}

func (a *API) setupUnauthenticatedRoutes() {
	a.router.GET("/", func(c *gin.Context) {
		a.htmlResponseWrapper(c, http.StatusOK, "index", gin.H{})
	})

	a.setupAPIEndpoints()
	a.setupAuthRoutes()
}

func (a *API) setupAuthenticatedRoutes() {
	a.setupUserRoutes()
	a.setupTeamRoutes()
	a.setupComputeRoutes()
	a.setupChartRoutes()
}

func (a *API) htmlResponseWrapper(c *gin.Context, status int, tmplName string, values gin.H) {
	values["loggedIn"] = a.isLoggedIn(c)
	values["isAdmin"] = a.isAdmin(c)

	c.HTML(status, tmplName, values)
}

func (a *API) isLoggedIn(c *gin.Context) bool {
	cookie, err := c.Cookie(sessionCookie)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return false
		}
		a.log.WithError(err).Error("reading session cookie")
		return false
	}

	return cookie != ""
}

func (a *API) isAdmin(c *gin.Context) bool {
	cookie, err := c.Cookie(sessionCookie)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return false
		}
		a.log.WithError(err).Error("reading session cookie")
		return false
	}

	session, err := a.repo.SessionGet(c, cookie)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false
		}
		a.log.WithError(err).Error("retrieving session from db")
		return false
	}

	return session.IsAdmin
}

func (a *API) fetchAdminGroupID() error {
	id, err := a.azureClient.GetGroupID(a.adminGroupMail)
	if err != nil {
		return fmt.Errorf("retrieve admin group id error: %v", err)
	}
	a.adminGroupID = id
	return nil
}
