package chart

import (
	"context"
	"fmt"
	"strings"

	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/database/gensql"
	"github.com/navikt/knorten/pkg/helm"
	"github.com/navikt/knorten/pkg/k8s"
	"github.com/navikt/knorten/pkg/reflect"
)

type JupyterConfigurableValues struct {
	TeamID     string
	UserIdents []string

	// User-configurable values
	CPULimit      string `helm:"singleuser.cpu.limit"`
	CPURequest    string `helm:"singleuser.cpu.guarantee"`
	MemoryLimit   string `helm:"singleuser.memory.limit"`
	MemoryRequest string `helm:"singleuser.memory.guarantee"`
	ImageName     string `helm:"singleuser.image.name"`
	ImageTag      string `helm:"singleuser.image.tag"`
	CullTimeout   string `helm:"cull.timeout"`
	AllowList     []string
}

type jupyterValues struct {
	// User-configurable values
	*JupyterConfigurableValues

	// Generated values
	CPULimit              string   `helm:"singleuser.cpu.limit"`
	CPUGuarantee          string   `helm:"singleuser.cpu.guarantee"`
	MemoryLimit           string   `helm:"singleuser.memory.limit"`
	MemoryGuarantee       string   `helm:"singleuser.memory.guarantee"`
	AllowedUsers          []string `helm:"hub.config.Authenticator.allowed_users"`
	OAuthCallbackURL      string   `helm:"hub.config.AzureAdOAuthenticator.oauth_callback_url"`
	KnadaTeamSecret       string   `helm:"singleuser.extraEnv.KNADA_TEAM_SECRET"`
	ProfileList           string   `helm:"singleuser.profileList"`
	ExtraAnnotations      string   `helm:"singleuser.extraAnnotations"`
	SingleUserExtraLabels string   `helm:"singleuser.extraLabels"`
}

const TeamValueKeyPYPIAccess = "pypiAccess,omit"

func (c Client) syncJupyter(ctx context.Context, configurableValues *JupyterConfigurableValues) error {
	team, err := c.repo.TeamGet(ctx, configurableValues.TeamID)
	if err != nil {
		return fmt.Errorf("retrieving team: %w", err)
	}

	values, err := c.jupyterMergeValues(ctx, team, configurableValues)
	if err != nil {
		return fmt.Errorf("merging values: %w", err)
	}

	namespace := k8s.TeamIDToNamespace(team.ID)

	if err := c.createHttpRoute(ctx, team.Slug+".jupyter."+c.topLevelDomain, namespace, gensql.ChartTypeJupyterhub); err != nil {
		return fmt.Errorf("creating http route: %w", err)
	}

	if err := c.createHealthCheckPolicy(ctx, namespace, gensql.ChartTypeJupyterhub); err != nil {
		return fmt.Errorf("creating health check policy: %w", err)
	}

	chartValues, err := reflect.CreateChartValues(values)
	if err != nil {
		return fmt.Errorf("creating chart values: %w", err)
	}

	err = c.repo.HelmChartValuesInsert(ctx, gensql.ChartTypeJupyterhub, chartValues, team.ID)
	if err != nil {
		return fmt.Errorf("saving chart values to database: %w", err)
	}

	return nil
}

// jupyterReleaseName creates a unique release name based on namespace name.
// The release name must be unique across namespaces as the helm chart creates a clusterrole
// for each jupyterhub with the same name as the release name
func jupyterReleaseName(namespace string) string {
	return fmt.Sprintf("%v-%v", string(gensql.ChartTypeJupyterhub), namespace)
}

func (c Client) jupyterMergeValues(ctx context.Context, team gensql.TeamGetRow, configurableValues *JupyterConfigurableValues) (jupyterValues, error) {
	if len(configurableValues.UserIdents) == 0 {
		err := c.repo.TeamConfigurableValuesGet(ctx, gensql.ChartTypeJupyterhub, team.ID, configurableValues)
		if err != nil {
			return jupyterValues{}, err
		}

		configurableValues.UserIdents, err = c.azureClient.ConvertEmailsToIdents(team.Users)
		if err != nil {
			return jupyterValues{}, err
		}
	}

	var profileList string
	if configurableValues.ImageName != "" {
		profileList = fmt.Sprintf(`[{"display_name":"Custom image","description":"Custom image for team %v","kubespawner_override":{"image":"%v:%v"}}]`,
			configurableValues.TeamID, configurableValues.ImageName, configurableValues.ImageTag)
	}

	var allowList string
	if len(configurableValues.AllowList) > 0 {
		allowList = fmt.Sprintf(`{"allowlist": "%v"}`, strings.Join(configurableValues.AllowList, ","))
	}

	singleuserExtraLabels := fmt.Sprintf(`{"team": "%v", "hub.jupyter.org/network-access-hub": "true"}`, team.ID)

	return jupyterValues{
		JupyterConfigurableValues: configurableValues,
		CPULimit:                  configurableValues.CPULimit,
		CPUGuarantee:              configurableValues.CPURequest,
		MemoryLimit:               configurableValues.MemoryLimit,
		MemoryGuarantee:           configurableValues.MemoryRequest,
		AllowedUsers:              configurableValues.UserIdents,
		OAuthCallbackURL:          fmt.Sprintf("https://%v.jupyter.%v/hub/oauth_callback", team.Slug, c.topLevelDomain),
		KnadaTeamSecret:           fmt.Sprintf("projects/%v/secrets/%v", c.gcpProject, team.ID),
		ProfileList:               profileList,
		ExtraAnnotations:          allowList,
		SingleUserExtraLabels:     singleuserExtraLabels,
	}, nil
}

func (c Client) deleteJupyter(ctx context.Context, teamID string) error {
	if err := c.repo.ChartDelete(ctx, teamID, gensql.ChartTypeJupyterhub); err != nil {
		return fmt.Errorf("deleting chart values: %w", err)
	}

	if c.dryRun {
		return nil
	}

	namespace := k8s.TeamIDToNamespace(teamID)

	if err := c.deleteHttpRoute(ctx, namespace, gensql.ChartTypeJupyterhub); err != nil {
		return fmt.Errorf("deleting http route: %w", err)
	}

	if err := c.deleteHealthCheckPolicy(ctx, namespace, gensql.ChartTypeJupyterhub); err != nil {
		return fmt.Errorf("deleting health check policy: %w", err)
	}

	return nil
}

func (c Client) registerJupyterHelmEvent(ctx context.Context, teamID string, eventType database.EventType) error {
	namespace := k8s.TeamIDToNamespace(teamID)
	helmEventData := helm.EventData{
		TeamID:       teamID,
		Namespace:    namespace,
		ReleaseName:  jupyterReleaseName(namespace),
		ChartType:    gensql.ChartTypeJupyterhub,
		ChartRepo:    "jupyterhub",
		ChartName:    "jupyterhub",
		ChartVersion: c.chartVersionJupyter,
	}

	if err := c.registerHelmEvent(ctx, eventType, teamID, helmEventData); err != nil {
		return err
	}

	return nil
}
