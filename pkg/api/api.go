package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nais/knorten/pkg/auth"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/google"
	"github.com/nais/knorten/pkg/helm"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/sirupsen/logrus"
)

type API struct {
	oauth2       *auth.Azure
	router       *gin.Engine
	helmClient   *helm.Client
	repo         *database.Repo
	log          *logrus.Entry
	googleClient *google.Google
	k8sClient    *k8s.Client
}

func New(repo *database.Repo, oauth2 *auth.Azure, helmClient *helm.Client, googleClient *google.Google, k8sClient *k8s.Client, log *logrus.Entry) (*API, error) {
	api := API{
		oauth2:       oauth2,
		helmClient:   helmClient,
		router:       gin.Default(),
		repo:         repo,
		googleClient: googleClient,
		k8sClient:    k8sClient,
		log:          log,
	}

	session, err := repo.NewSessionStore()
	if err != nil {
		return &API{}, err
	}
	api.router.Use(session)
	api.router.Static("/assets", "./assets")
	api.router.LoadHTMLGlob("templates/**/*")
	api.setupUnauthenticatedRoutes()
	api.router.Use(api.authMiddleware())
	api.setupAuthenticatedRoutes()
	return &api, nil
}

func (a *API) Run() error {
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
	a.setupAdminRoutes()
	a.setupUserRoutes()
	a.setupTeamRoutes()
	a.setupChartRoutes()
}
