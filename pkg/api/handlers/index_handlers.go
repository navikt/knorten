package handlers

import (
	"net/http"

	"github.com/navikt/knorten/pkg/api/middlewares"

	"github.com/gin-gonic/gin"
)

func IndexHandler(ctx *gin.Context) {
	ctx.HTML(http.StatusOK, "index", gin.H{
		"loggedIn": ctx.GetBool(middlewares.LoggedInKey),
		"isAdmin":  ctx.GetBool(middlewares.AdminKey),
	})
}
