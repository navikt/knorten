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

	a.router.GET("/oauth2/login", func(c *gin.Context) {
		fmt.Println("login")
		consentURL := a.Login(c)
		c.Redirect(http.StatusSeeOther, consentURL)
	})

	a.router.GET("/oauth2/callback", func(c *gin.Context) {
		redirectURL, err := a.Callback(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}

		c.Redirect(http.StatusSeeOther, redirectURL)
	})

	a.router.GET("/oauth2/logout", func(c *gin.Context) {
		redirectURL, err := a.Logout(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		c.Redirect(http.StatusSeeOther, redirectURL)
	})
}

func (a *API) setupAuthenticatedRoutes() {
	a.router.GET("/user", func(c *gin.Context) {
		var user *auth.User
		anyUser, exists := c.Get("user")
		if exists {
			user = anyUser.(*auth.User)
		}

		get, err := a.repo.UserAppsGet(c, user.Email)
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

	a.router.GET("/team/new", func(c *gin.Context) {
		var form chart.NamespaceForm
		// err := c.ShouldBind(&form)
		c.HTML(http.StatusOK, "charts/namespace.tmpl", gin.H{
			"form": form,
		})
	})

	a.router.POST("/team/new", func(c *gin.Context) {
		err := chart.CreateNamespace(c, a.repo, a.helmClient, gensql.ChartTypeNamespace, a.dryRun)
		if err != nil {
			fmt.Println(err)
			// c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Redirect(http.StatusSeeOther, "/team/new")
		}
		c.Redirect(http.StatusSeeOther, "/user")
	})

	a.router.GET("/team/:team/edit", func(c *gin.Context) {
		team := c.Param("team")
		namespaceForm := &chart.NamespaceForm{}
		err := a.repo.TeamConfigurableValuesGet(c, gensql.ChartTypeNamespace, team, namespaceForm)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.HTML(http.StatusOK, "charts/namespace.tmpl", gin.H{
			"values": namespaceForm,
			"team":   team,
		})
	})

	a.router.POST("/team/:team/edit", func(c *gin.Context) {
		err := chart.UpdateNamespace(c, a.repo)
		if err != nil {
			fmt.Println(err)
			team := c.Param("team")
			// c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/new", team))
			return
		}
		c.Redirect(http.StatusSeeOther, "/user")
	})

	a.router.GET("/chart/:chart/new", func(c *gin.Context) {
		chartType := strings.ToLower(c.Param("chart"))
		var form chart.JupyterForm
		err := c.ShouldBind(&form)
		fmt.Println(err)
		fmt.Println(form)
		c.HTML(http.StatusOK, fmt.Sprintf("charts/%v.tmpl", chartType), gin.H{
			"form": form,
		})
	})

	a.router.POST("/chart/:chart/new", func(c *gin.Context) {
		chartType := getChartType(c.Param("chart"))

		switch chartType {
		case gensql.ChartTypeJupyterhub:
			err := chart.CreateJupyterhub(c, a.repo, a.helmClient)
			if err != nil {
				fmt.Println(err)
				// c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				c.Redirect(http.StatusSeeOther, "/chart/jupyterhub/new")
			}
			c.Redirect(http.StatusSeeOther, "/user")
		case gensql.ChartTypeAirflow:
			err := chart.CreateAirflow(c, a.repo, chartType)
			if err != nil {
				fmt.Println(err)
				// c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				c.Redirect(http.StatusSeeOther, "/chart/airflow/new")
			}
			c.Redirect(http.StatusSeeOther, "/user")

		}
	})

	a.router.GET("/chart/:chart/:namespace/edit", func(c *gin.Context) {
		chartType := getChartType(c.Param("chart"))
		namespace := c.Param("namespace")

		switch chartType {
		case gensql.ChartTypeJupyterhub:
			configurableValues := &chart.JupyterConfigurableValues{}
			err := a.repo.TeamConfigurableValuesGet(c, chartType, namespace, configurableValues)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.HTML(http.StatusOK, fmt.Sprintf("charts/%v.tmpl", chartType), gin.H{
				"values":    configurableValues,
				"namespace": namespace,
			})
		case gensql.ChartTypeAirflow:
		}
	})

	a.router.POST("/chart/:chart/:namespace/edit", func(c *gin.Context) {
		chartType := getChartType(c.Param("chart"))

		switch chartType {
		case gensql.ChartTypeJupyterhub:
			err := chart.UpdateJupyterhub(c, a.repo, a.helmClient)
			if err != nil {
				fmt.Println(err)
				// c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				c.Redirect(http.StatusSeeOther, "/chart/jupyterhub/new")
				return
			}
			c.Redirect(http.StatusSeeOther, "/user")
		case gensql.ChartTypeAirflow:
			err := chart.CreateAirflow(c, a.repo, chartType)
			if err != nil {
				fmt.Println(err)
				// c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				c.Redirect(http.StatusSeeOther, "/chart/airflow/new")
			}
			c.Redirect(http.StatusSeeOther, "/user")

		}
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
