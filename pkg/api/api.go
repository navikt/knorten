package api

import (
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
	dryRun              bool
}

func New(repo *database.Repo, azureClient *auth.Azure, googleClient *google.Google, k8sClient *k8s.Client, cryptClient *crypto.EncrypterDecrypter, dryRun bool, airflowChartVersion, jupyterChartVersion, sessionKey string, log *logrus.Entry) (*gin.Engine, error) {
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
		azureClient:  azureClient,
		router:       router,
		repo:         repo,
		googleClient: googleClient,
		k8sClient:    k8sClient,
		adminClient:  adminClient,
		cryptClient:  cryptClient,
		log:          log,
		chartClient:  chartClient,
		dryRun:       dryRun,
	}

	api.teamClient = team.NewClient(repo, googleClient, k8sClient, api.chartClient, log)

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
		c.HTML(http.StatusOK, "index", gin.H{
			"current": "home",
		})
	})

	a.setupAPIEndpoints()
	a.setupAuthRoutes()
}

func (a *API) setupAuthenticatedRoutes() {
	a.setupUserRoutes()
	a.setupTeamRoutes()
	a.setupChartRoutes()
}
