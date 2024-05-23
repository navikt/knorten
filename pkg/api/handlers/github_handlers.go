package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/navikt/knorten/pkg/api/service"
	"github.com/navikt/knorten/pkg/github"
	"github.com/sirupsen/logrus"
)

type GithubHandler struct {
	githubService service.GithubService
	log           *logrus.Entry
}

func NewGithubHandler(githubService service.GithubService, log *logrus.Entry) *GithubHandler {
	return &GithubHandler{
		githubService: githubService,
		log:           log,
	}
}

func (h *GithubHandler) RepositoriesHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, h.githubService.Repositories(ctx))
	}
}

func (h *GithubHandler) BranchesHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		owner := ctx.Param("owner")
		repo := ctx.Param("repo")
		slug := fmt.Sprintf("%v/%v", owner, repo)

		branches, err := h.githubService.Branches(ctx, github.Repository{
			FullName: slug,
			Name:     repo,
		})
		if err != nil {
			h.log.WithField("repository", slug).WithError(err).Error("loading branches from github")
			ctx.JSON(http.StatusInternalServerError, map[string]string{
				"status":  strconv.Itoa(http.StatusInternalServerError),
				"message": "Internal server error",
			})

			return
		}

		ctx.JSON(http.StatusOK, gin.H{"branches": branches})
	}
}
