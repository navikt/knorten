package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/nais/knorten/pkg/auth"
	"github.com/nais/knorten/pkg/database/gensql"
	"net/http"
)

func createIngress(team string, chartType gensql.ChartType) string {
	switch chartType {
	case gensql.ChartTypeJupyterhub:
		return fmt.Sprintf("https://%v.jupyter.knada.io", team)
	case gensql.ChartTypeAirflow:
		return fmt.Sprintf("https://%v.airflow.knada.io", team)
	}

	return ""
}

func (a *API) setupUserRoutes() {
	a.router.GET("/user", func(c *gin.Context) {
		var user *auth.User
		anyUser, exists := c.Get("user")
		if exists {
			user = anyUser.(*auth.User)
		}

		get, err := a.repo.ServicesForUser(c, user.Email)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var services []Service
		for team, apps := range get {
			for _, a := range apps {
				services = append(services, Service{
					App:            string(a),
					Ingress:        createIngress(team, a),
					Namespace:      team,
					Secret:         fmt.Sprintf("https://console.cloud.google.com/security/secret-manager/secret/%v/versions?project=knada-gcp", team),
					ServiceAccount: fmt.Sprintf("%v@knada-gcp.iam.gserviceaccount.com", team),
				})
			}
		}
		c.HTML(http.StatusOK, "user/index.tmpl", gin.H{
			"user":   c.Param("logged in user"),
			"charts": services,
		})
	})
}
