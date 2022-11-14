package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/nais/knorten/pkg/auth"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"net/http"
)

type Service struct {
	App            string
	Ingress        string
	Namespace      string
	Secret         string
	ServiceAccount string
}

type Services struct {
	Jupyterhub *Service
	Airflow    *Service
}

func createIngress(team string, chartType gensql.ChartType) string {
	switch chartType {
	case gensql.ChartTypeJupyterhub:
		return fmt.Sprintf("https://%v.jupyter.knada.io", team)
	case gensql.ChartTypeAirflow:
		return fmt.Sprintf("https://%v.airflow.knada.io", team)
	}

	return ""
}

func createService(team string, chartType gensql.ChartType) *Service {
	return &Service{
		App:            string(chartType),
		Ingress:        createIngress(team, chartType),
		Namespace:      team,
		Secret:         fmt.Sprintf("https://console.cloud.google.com/security/secret-manager/secret/%v/versions?project=knada-gcp", team),
		ServiceAccount: fmt.Sprintf("%v@knada-gcp.iam.gserviceaccount.com", team),
	}
}

func createServiceSidebar(c *gin.Context, repo *database.Repo) (map[string]Services, error) {
	var user *auth.User
	anyUser, exists := c.Get("user")
	if exists {
		user = anyUser.(*auth.User)
	}

	get, err := repo.ServicesForUser(c, user.Email)
	if err != nil {
		return map[string]Services{}, err
	}

	services := map[string]Services{}
	for team, apps := range get {
		service := Services{}
		for _, app := range apps {
			switch app {
			case gensql.ChartTypeJupyterhub:
				service.Jupyterhub = createService(team, app)
			case gensql.ChartTypeAirflow:
				service.Airflow = createService(team, app)
			}
		}
		services[team] = service
	}

	return services, nil
}

func (a *API) setupUserRoutes() {
	a.router.GET("/user", func(c *gin.Context) {
		services, err := createServiceSidebar(c, a.repo)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.HTML(http.StatusOK, "user/index", gin.H{
			"services": services,
		})
	})
}
