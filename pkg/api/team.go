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
		c.HTML(http.StatusOK, "charts/team.tmpl", gin.H{
			"form": form,
		})
	})

	a.router.POST("/team/new", func(c *gin.Context) {
		err := team.Create(c, a.repo, a.googleClient, a.k8sClient)
		if err != nil {
			fmt.Println(err)
			// c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Redirect(http.StatusSeeOther, "/team/new")
		}
		c.Redirect(http.StatusSeeOther, "/user")
	})

	//a.router.GET("/team/:team/edit", func(c *gin.Context) {
	//	teamName := c.Param("team")
	//	namespaceForm := &team.Form{}
	//	err := a.repo.TeamConfigurableValuesGet(c, gensql.ChartTypeNamespace, teamName, namespaceForm)
	//	if err != nil {
	//		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	//		return
	//	}
	//	c.HTML(http.StatusOK, "charts/team.tmpl", gin.H{
	//		"values": namespaceForm,
	//		"team":   teamName,
	//	})
	//})

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
