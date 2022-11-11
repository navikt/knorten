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

		fmt.Println(user)
		get, err := a.repo.ServicesForUser(c, user.Email)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		fmt.Println(get)
		services := map[string][]Service{}
		for team, apps := range get {
			fmt.Println(team, apps)
			services[team] = []Service{}
			for _, app := range apps {
				services[team] = append(services[team], Service{
					App:            string(app),
					Ingress:        createIngress(team, app),
					Namespace:      team,
					Secret:         fmt.Sprintf("https://console.cloud.google.com/security/secret-manager/secret/%v/versions?project=knada-gcp", team),
					ServiceAccount: fmt.Sprintf("%v@knada-gcp.iam.gserviceaccount.com", team),
				})
			}
		}
		c.HTML(http.StatusOK, "user/index.tmpl", gin.H{
			"current": "user",
			"user":    c.Param("logged in user"),
			"charts":  services,
		})
	})
}
