package api

import (
	"github.com/gin-gonic/gin"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/helm"
)

type API struct {
	router     *gin.Engine
	helmClient helm.Client
	repo       *database.Repo
}

func New() *API {
	api := API{
		router: gin.Default(),
	}

	api.setupRouter()
	return &api
}

func (a *API) Run() error {
	return a.router.Run()
}

func (a *API) setupRouter() {
	a.router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Knorten says 'Hi'",
		})
	})

	a.router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	a.router.POST("/", func(c *gin.Context) {
		releaseName := "team-nada-jupyterhub"
		team := "team-nada"
		jupyterhub := helm.NewJupyterhub("nada", "charts/jupyterhub/values.yaml", a.repo)
		a.helmClient.InstallOrUpgrade(releaseName, team, jupyterhub)
	})
}
