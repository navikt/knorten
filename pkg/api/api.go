package api

import (
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
	oauth2       *auth.Azure
	router       *gin.Engine
	helmClient   *helm.Client
	repo         *database.Repo
	log          *logrus.Entry
	googleClient *google.Google
	k8sClient    *k8s.Client
	adminClient  *admin.Client
	cryptor      *crypto.EncrypterDecrypter
	dryRun       bool
}

func New(repo *database.Repo, oauth2 *auth.Azure, helmClient *helm.Client, googleClient *google.Google, k8sClient *k8s.Client, cryptor *crypto.EncrypterDecrypter, log *logrus.Entry, dryRun bool) (*API, error) {
	adminClient := admin.New(repo, helmClient, cryptor)
	api := API{
		oauth2:       oauth2,
		helmClient:   helmClient,
		router:       gin.Default(),
		repo:         repo,
		googleClient: googleClient,
		k8sClient:    k8sClient,
		adminClient:  adminClient,
		cryptor:      cryptor,
		log:          log,
		dryRun:       dryRun,
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

func (a *API) setupAuthenticatedAdminRoutes() {
	a.setupAdminRoutes()
}
