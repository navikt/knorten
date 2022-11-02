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
	"strings"
)

type API struct {
	router     *gin.Engine
	helmClient helm.Client
	repo       *database.Repo
}

type Service struct {
	App            string
	Ingress        string
	Namespace      string
	Secret         string
	ServiceAccount string
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
	Namespace string   `form:"namespace"`
	Users     []string `form:"users[]" binding:"required"`
	CPU       int      `form:"cpu"`
	Memory    string   `form:"memory"`
}

func CreateIngress(team string, chartType gensql.ChartType) string {
	switch chartType {
	case gensql.ChartTypeJupyterhub:
		return fmt.Sprintf("https://%v.jupyter.knada.io", team)
	case gensql.ChartTypeAirflow:
		return fmt.Sprintf("https://%v.airflow.knada.io", team)
	}

	return ""
}

func (a *API) setupRouter() {
	a.router.LoadHTMLGlob("templates/**/*")

	a.router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"title": "Knorten",
		})
	})

	a.router.GET("/user", func(c *gin.Context) {
		get, err := a.repo.UserAppsGet(context.Background(), "kyrre.havik@nav.no")
		if err != nil {
			return
		}
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var services []Service
		for _, row := range get {
			services = append(services, Service{
				App:            string(row.ChartType),
				Ingress:        CreateIngress(row.Team, row.ChartType),
				Namespace:      row.Team,
				Secret:         fmt.Sprintf("https://console.cloud.google.com/security/secret-manager/secret/%v/versions?project=knada-gcp", row.Team),
				ServiceAccount: fmt.Sprintf("%v@knada-gcp.iam.gserviceaccount.com", row.Team),
			})
		}
		c.HTML(http.StatusOK, "user/index.tmpl", gin.H{
			"user":   c.Param("logged in user"),
			"charts": services,
		})
	})

	a.router.GET("/chart/:chart/new", func(c *gin.Context) {
		chart := strings.ToLower(c.Param("chart"))
		c.HTML(http.StatusOK, fmt.Sprintf("charts/%v.tmpl", chart), gin.H{})
	})

	a.router.GET("/chart/:chart/:owner/edit", func(c *gin.Context) {
		chart := strings.ToLower(c.Param("chart"))
		owner := c.Param("owner")
		c.HTML(http.StatusOK, fmt.Sprintf("charts/%v.tmpl", chart), gin.H{
			"owner": owner,
		})
	})

	a.router.POST("/chart/:chart/new", func(c *gin.Context) {
		var form Jupyter
		c.ShouldBind(&form)
		chart := c.Param("chart")

		if err := c.ShouldBindWith(&form, binding.Query); err == nil {

			err := a.repo.TeamChartValueInsert(context.Background(), "singleuser.image.name", form.Memory, form.Namespace, gensql.ChartTypeJupyterhub)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{"form": form, "chart": chart})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}

		//c.HTML(http.StatusOK, fmt.Sprintf("user/new_%v.tmpl", chart), gin.H{
		//	"user":   c.Param("user"),
		//	"user":   form.Users,
		//	"cpu":    form.CPU,
		//	"memory": form.Memory,
		//})
	})

	a.router.POST("/", func(c *gin.Context) {
		releaseName := "user-nada-jupyterhub"
		team := "user-nada"
		jupyterhub := helm.NewJupyterhub("nada", "charts/jupyterhub/values.yaml", a.repo)
		a.helmClient.InstallOrUpgrade(releaseName, team, jupyterhub)
	})
}
