package api

import (
	"github.com/gin-gonic/gin"
	"github.com/nais/knorten/pkg/auth"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/helm"
	"github.com/sirupsen/logrus"
	"net/http"
)

type API struct {
	oauth2     *auth.Azure
	router     *gin.Engine
	helmClient *helm.Client
	repo       *database.Repo
	log        *logrus.Entry
	dryRun     bool
}

type Service struct {
	App            string
	Ingress        string
	Namespace      string
	Secret         string
	ServiceAccount string
}

func New(repo *database.Repo, oauth2 *auth.Azure, helmClient *helm.Client, log *logrus.Entry, dryRun bool) *API {
	api := API{
		oauth2:     oauth2,
		helmClient: helmClient,
		router:     gin.Default(),
		repo:       repo,
		log:        log,
		dryRun:     dryRun,
	}

	api.router.Static("/assets", "./assets")
	api.router.LoadHTMLGlob("templates/**/*")
	api.setupUnauthenticatedRoutes()
	api.router.Use(api.authMiddleware())
	api.setupAuthenticatedRoutes()
	return &api
}

func (a *API) Run() error {
	return a.router.Run()
}

func (a *API) setupUnauthenticatedRoutes() {
	a.router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"title": "Knorten",
		})
	})

	a.setupAuthRoutes()
}

func (a *API) setupAuthenticatedRoutes() {
	a.setupUserRoutes()
	a.setupTeamRoutes()
	a.setupChartRoutes()
}
