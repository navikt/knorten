package api

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/helm"
	"net/http"
)

type API struct {
	router     *gin.Engine
	helmClient helm.Client
	repo       *database.Repo
}

func New(repo *database.Repo) *API {
	api := API{
		router: gin.Default(),
		repo:   repo,
	}

	api.router.Static("/assets", "./assets")
	api.setupRouter()
	return &api
}

func (a *API) Run() error {
	return a.router.Run()
}

type Jupyter struct {
	Image   string   `chart:"singleuser.image.name"`
	Sidecar string   `chart:""`
	Users   []string `form:"users[]" binding:"required"`
	CPU     int      `form:"cpu"`
	Memory  string   `form:"memory"`
}

func (a *API) setupRouter() {
	a.router.LoadHTMLGlob("templates/**/*")

	a.router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"title": "Knorten",
		})
	})

	a.router.GET("/team/:team", func(c *gin.Context) {
		c.HTML(http.StatusOK, "team/index.tmpl", gin.H{
			"team":   c.Param("team"),
			"charts": []string{"Jupyter", "Airflow"},
		})
	})

	a.router.GET("/team/:team/new/:chart", func(c *gin.Context) {
		chart := c.Param("chart")
		c.HTML(http.StatusOK, fmt.Sprintf("team/new_%v.tmpl", chart), gin.H{
			"team": c.Param("team"),
		})
	})

	a.router.POST("/team/:team/new/:chart", func(c *gin.Context) {
		var form Jupyter
		c.ShouldBind(&form)
		team := c.Param("team")
		chart := c.Param("chart")

		if err := c.ShouldBindWith(&form, binding.Query); err == nil {
			err := a.repo.TeamChartValueInsert(context.Background(), "singleuser.image.name", form.Memory, team, gensql.ChartTypeJupyterhub)
			if err != nil {
				return
			}

			c.JSON(http.StatusOK, gin.H{"form": form, "chart": chart})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}

		//c.HTML(http.StatusOK, fmt.Sprintf("team/new_%v.tmpl", chart), gin.H{
		//	"team":   c.Param("team"),
		//	"user":   form.Users,
		//	"cpu":    form.CPU,
		//	"memory": form.Memory,
		//})
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
