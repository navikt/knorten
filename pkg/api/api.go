package api

import (
	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/team"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nais/knorten/pkg/admin"
	"github.com/nais/knorten/pkg/auth"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/crypto"
	"github.com/nais/knorten/pkg/google"
	"github.com/nais/knorten/pkg/helm"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/sirupsen/logrus"
)

type API struct {
	azureClient  *auth.Azure
	router       *gin.Engine
	helmClient   *helm.Client
	repo         *database.Repo
	log          *logrus.Entry
	googleClient *google.Google
	k8sClient    *k8s.Client
	adminClient  *admin.Client
	cryptClient  *crypto.EncrypterDecrypter
	dryRun       bool
	chartClient  *chart.Client
	teamClient   *team.Client
}

func New(repo *database.Repo, azureClient *auth.Azure, helmClient *helm.Client, googleClient *google.Google, k8sClient *k8s.Client, cryptClient *crypto.EncrypterDecrypter, log *logrus.Entry, dryRun bool) (*API, error) {
	adminClient := admin.New(repo, helmClient, cryptClient)
	api := API{
		azureClient:  azureClient,
		helmClient:   helmClient,
		router:       gin.Default(),
		repo:         repo,
		googleClient: googleClient,
		k8sClient:    k8sClient,
		adminClient:  adminClient,
		cryptClient:  cryptClient,
		log:          log,
		dryRun:       dryRun,
		chartClient: &chart.Client{
			Airflow:    chart.NewAirflowClient(repo, googleClient, k8sClient, helmClient, cryptClient, log),
			Jupyterhub: chart.NewJupyterhubClient(repo, helmClient, cryptClient, log),
		},
	}

	api.teamClient = team.NewClient(repo, googleClient, helmClient, k8sClient, api.chartClient, log)

	session, err := repo.NewSessionStore()
	if err != nil {
		return &API{}, err
	}

	api.router.Use(session)
	api.router.Static("/assets", "./assets")
	api.router.LoadHTMLGlob("templates/**/*")
	api.setupUnauthenticatedRoutes()
	api.router.Use(api.authMiddleware([]string{}))
	api.setupAuthenticatedRoutes()
	api.router.Use(api.authMiddleware([]string{"kyrre.havik@nav.no", "erik.vattekar@nav.no"}))
	api.setupAdminRoutes()
	return &api, nil
}

func (a *API) Run() error {
	if a.dryRun {
		return a.router.Run("localhost:8080")
	}

	return a.router.Run()
}

func (a *API) setupUnauthenticatedRoutes() {
	a.router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index", gin.H{
			"current": "home",
		})
	})

	a.setupAuthRoutes()
}

func (a *API) setupAuthenticatedRoutes() {
	a.setupUserRoutes()
	a.setupTeamRoutes()
	a.setupChartRoutes()
}
