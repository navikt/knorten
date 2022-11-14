package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nais/knorten/pkg/team"
)

func (a *API) setupTeamRoutes() {
	a.router.GET("/team/new", func(c *gin.Context) {
		var form team.Form
		services, err := createServiceSidebar(c, a.repo)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.HTML(http.StatusOK, "team/new", gin.H{
			"form":     form,
			"services": services,
		})
	})

	a.router.POST("/team/new", func(c *gin.Context) {
		err := team.Create(c, a.repo, a.googleClient, a.k8sClient)
		if err != nil {
			// TODO: Bruke middleware/session for å legge feilmeldinger til http-kallet
			// c.Error()
			fmt.Println(err)
			// c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Redirect(http.StatusSeeOther, "/team/new")
			return
		}
		c.Redirect(http.StatusSeeOther, "/user")
	})

	a.router.GET("/team/:team/edit", func(c *gin.Context) {
		teamName := c.Param("team")
		get, err := a.repo.TeamGet(c, teamName)
		if err != nil {
			fmt.Println(err)
			// c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Redirect(http.StatusSeeOther, "/team/new")
			return
		}
		c.HTML(http.StatusOK, "team/edit", gin.H{
			"users": get.Users,
			"team":  teamName,
		})
	})

	//a.router.POST("/team/:team/edit", func(c *gin.Context) {
	//	err := chart.UpdateNamespace(c, a.helmClient, a.repo)
	//	if err != nil {
	//		fmt.Println(err)
	//		team := c.Param("team")
	//		// c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	//		c.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/new", team))
	//		return
	//	}
	//	c.Redirect(http.StatusSeeOther, "/user")
	//})
}
