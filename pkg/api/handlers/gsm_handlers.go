package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/navikt/knorten/pkg/api/middlewares"
	"github.com/navikt/knorten/pkg/api/service"
	"github.com/navikt/knorten/pkg/teamsecrets"
	"github.com/sirupsen/logrus"
)

type GSMHandler struct {
	defaultGSMProject string
	gsmService        service.GSMService
	log               *logrus.Entry
}

func NewGSMHandler(defaultGSMProject string, gsmService service.GSMService, log *logrus.Entry) *GSMHandler {
	return &GSMHandler{
		defaultGSMProject: defaultGSMProject,
		gsmService:        gsmService,
		log:               log,
	}
}

func (g *GSMHandler) TeamSecretGroups() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		teamSlug := ctx.Param("slug")

		secretGroups, err := g.gsmService.GetTeamSecretGroups(ctx, &g.defaultGSMProject, teamSlug)
		if err != nil {
			g.log.Errorf("problem getting secret groups for team %v: %v", teamSlug, err)
		}

		ctx.HTML(http.StatusOK, "secrets/index", gin.H{
			"secrets":  secretGroups,
			"slug":     teamSlug,
			"loggedIn": ctx.GetBool(middlewares.LoggedInKey),
			"isAdmin":  ctx.GetBool(middlewares.AdminKey),
		})
	}
}

func (h *GSMHandler) CreateOrUpdateTeamSecretGroup() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		teamSlug := ctx.Param("slug")
		secretGroup := ctx.Param("group")

		if err := ctx.Request.ParseForm(); err != nil {
			h.log.Errorf("problem getting team from slug %v: %v", teamSlug, err)
		}

		groupSecrets, err := groupSecretsFromForm(ctx, h.defaultGSMProject)
		if err != nil {
			h.log.Errorf("creating or updating team secret group %v for team %v: %v", secretGroup, teamSlug, err)
		}

		if err := h.gsmService.CreateOrUpdateTeamSecretGroup(ctx, &h.defaultGSMProject, teamSlug, teamsecrets.FormatGroupName(secretGroup), groupSecrets); err != nil {
			h.log.Errorf("creating or updating team secret group %v for team %v: %v", secretGroup, teamSlug, err)
		}

		ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/secrets", teamSlug))
	}
}

func (h *GSMHandler) DeleteTeamSecretGroup() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		teamSlug := ctx.Param("slug")
		secretGroup := ctx.Param("group")

		if err := h.gsmService.DeleteTeamSecretGroup(ctx, &h.defaultGSMProject, teamSlug, secretGroup); err != nil {
			h.log.Errorf("creating or updating team secret group %v for team %v: %v", secretGroup, teamSlug, err)
		}
		ctx.Redirect(http.StatusSeeOther, fmt.Sprintf("/team/%v/secrets", teamSlug))
	}
}

func groupSecretsFromForm(ctx *gin.Context, gcpProject string) ([]teamsecrets.TeamSecret, error) {
	groupSecrets := []teamsecrets.TeamSecret{}
	for key, value := range ctx.Request.PostForm {
		if strings.HasPrefix(key, "key.") {
			key, value, err := findValueForKey(key, value[0], ctx.Request.PostForm)
			if err != nil {
				return nil, err
			}
			groupSecrets = append(groupSecrets, teamsecrets.TeamSecret{
				Key:   fmt.Sprintf("projects/%v/secrets/%v", gcpProject, teamsecrets.FormatSecretName(key)),
				Name:  teamsecrets.FormatSecretName(key),
				Value: value[0],
			})
		}
	}
	return groupSecrets, nil
}

func findValueForKey(keyID, keyValue string, formData url.Values) (string, []string, error) {
	matchOn := strings.TrimPrefix(keyID, "key.")

	for k, v := range formData {
		if strings.HasPrefix(k, "value.") && strings.Contains(k, matchOn) {
			return keyValue, v, nil
		}
	}

	return "", nil, fmt.Errorf("error parsing new secret key value pair for keyID %v", keyID)
}
