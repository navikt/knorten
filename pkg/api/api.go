package api

import (
	"github.com/nais/knorten/pkg/chart"
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

type chartClients struct {
	Airflow    chart.AirflowClient
	Jupyterhub chart.JupyterhubClient
}

type API struct {
	offline      bool
	oauth2       *auth.Azure
	router       *gin.Engine
	helmClient   *helm.Client
	repo         *database.Repo
	log          *logrus.Entry
	googleClient *google.Google
	k8sClient    *k8s.Client
	adminClient  *admin.Client
	cryptoClient *crypto.EncrypterDecrypter
	chart        chartClients
}

func New(repo *database.Repo, oauth2 *auth.Azure, helmClient *helm.Client, googleClient *google.Google, k8sClient *k8s.Client, cryptoClient *crypto.EncrypterDecrypter, log *logrus.Entry, offline bool) (*API, error) {
	adminClient := admin.New(repo, helmClient, cryptoClient)
	api := API{
		oauth2:       oauth2,
		helmClient:   helmClient,
		router:       gin.Default(),
		repo:         repo,
		googleClient: googleClient,
		k8sClient:    k8sClient,
		adminClient:  adminClient,
		cryptoClient: cryptoClient,
		log:          log,
		offline:      offline,
	}

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
	if a.offline {
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
