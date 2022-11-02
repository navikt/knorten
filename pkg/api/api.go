package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nais/knorten/pkg/api/chart"
	"github.com/nais/knorten/pkg/auth"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/helm"
	"github.com/sirupsen/logrus"
)

type API struct {
	oauth2     *auth.Azure
	router     *gin.Engine
	helmClient helm.Client
	repo       *database.Repo
	log        *logrus.Entry
}

type Service struct {
	App            string
	Ingress        string
	Namespace      string
	Secret         string
	ServiceAccount string
}

func New(repo *database.Repo, oauth2 *auth.Azure, log *logrus.Entry) *API {
	api := API{
		oauth2: oauth2,
		router: gin.Default(),
		repo:   repo,
		log:    log,
	}

	api.router.Static("/assets", "./assets")
	api.setupUnauthenticatedRoutes()
	api.router.Use(api.authMiddleware())
	api.setupAuthenticatedRoutes()
	// api.setupRouter()
	return &api
}

func (a *API) Run() error {
	return a.router.Run()
}

func (a *API) setupAuthenticatedRoutes() {
	a.router.GET("/testy", func(c *gin.Context) {
		fmt.Println("authenticated endpoint")
		fmt.Println(c.Get("user"))
	})
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

func getChartType(chartType string) gensql.ChartType {
	switch chartType {
	case string(gensql.ChartTypeJupyterhub):
		return gensql.ChartTypeJupyterhub
	case string(gensql.ChartTypeAirflow):
		return gensql.ChartTypeAirflow
	default:
		return ""
	}
}

func (a *API) setupRouter() {
	a.router.LoadHTMLGlob("templates/**/*")

	a.router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"title": "Knorten",
		})
	})

	a.router.GET("/user", func(c *gin.Context) {
		get, err := a.repo.UserAppsGet(c, "kyrre.havik@nav.no")
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

	a.router.POST("/chart/:chart/new", func(c *gin.Context) {
		chartType := getChartType(c.Param("chart"))

		switch chartType {
		case gensql.ChartTypeJupyterhub:
			err := chart.CreateJupyterhub(c, a.repo, chartType)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			}
			c.Redirect(http.StatusSeeOther, "/user")
		}
	})

	a.router.GET("/chart/:chart/:owner/edit", func(c *gin.Context) {
		chart := strings.ToLower(c.Param("chart"))
		owner := c.Param("owner")
		c.HTML(http.StatusOK, fmt.Sprintf("charts/%v.tmpl", chart), gin.H{
			"owner": owner,
		})
	})

	a.router.POST("/", func(c *gin.Context) {
		releaseName := "user-nada-jupyterhub"
		team := "user-nada"
		jupyterhub := helm.NewJupyterhub("nada", "charts/jupyterhub/values.yaml", a.repo)
		a.helmClient.InstallOrUpgrade(releaseName, team, jupyterhub)
	})
}
