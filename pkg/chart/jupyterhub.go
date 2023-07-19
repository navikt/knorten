package chart

import (
	"context"
	"fmt"
	"strings"

	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/helm"
	helmApps "github.com/nais/knorten/pkg/helm/applications"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/nais/knorten/pkg/reflect"
)

type JupyterConfigurableValues struct {
	Slug       string
	UserIdents []string

	// User-configurable values
	CPU         string
	Memory      string
	ImageName   string `helm:"singleuser.image.name"`
	ImageTag    string `helm:"singleuser.image.tag"`
	CullTimeout uint64 `helm:"cull.timeout"`
	AllowList   []string
}

func (v *JupyterConfigurableValues) MemoryWithoutUnit() string {
	if v.Memory == "" {
		return ""
	}
	return v.Memory[:len(v.Memory)-1]
}

type jupyterValues struct {
	// User-configurable values
	JupyterConfigurableValues

	// Generated values
	CPULimit         string   `helm:"singleuser.cpu.limit"`
	CPUGuarantee     string   `helm:"singleuser.cpu.guarantee"`
	MemoryLimit      string   `helm:"singleuser.memory.limit"`
	MemoryGuarantee  string   `helm:"singleuser.memory.guarantee"`
	AdminUsers       []string `helm:"hub.config.Authenticator.admin_users"`
	AllowedUsers     []string `helm:"hub.config.Authenticator.allowed_users"`
	Hosts            string   `helm:"ingress.hosts"`
	IngressTLS       string   `helm:"ingress.tls"`
	OAuthCallbackURL string   `helm:"hub.config.AzureAdOAuthenticator.oauth_callback_url"`
	KnadaTeamSecret  string   `helm:"singleuser.extraEnv.KNADA_TEAM_SECRET"`
	ProfileList      string   `helm:"singleuser.profileList"`
	ExtraAnnotations string   `helm:"singleuser.extraAnnotations"`
}

// TODO: Trenger en sync som henter ut eksisterende verdier fra databasen

func (c Client) syncJupyter(ctx context.Context, configurableValues JupyterConfigurableValues) error {
	team, err := c.repo.TeamGet(ctx, configurableValues.Slug)
	if err != nil {
		return err
	}

	values := jupyterMergeValues(team, configurableValues)

	chartValues, err := reflect.CreateChartValues(values)
	if err != nil {
		return err
	}

	err = c.repo.TeamValuesInsert(ctx, gensql.ChartTypeJupyterhub, chartValues, team.ID)
	if err != nil {
		return err
	}

	application := helmApps.NewJupyterhub(team.ID, c.repo, c.chartVersionJupyter)
	charty, err := application.Chart(ctx)
	if err != nil {
		return err
	}

	namespace := k8s.TeamIDToNamespace(team.ID)
	releaseName := jupyterReleaseName(namespace)
	return helm.InstallOrUpgrade(releaseName, c.chartVersionJupyter, namespace, charty.Values)
}

func (c Client) deleteJupyter(ctx context.Context, teamID string) error {
	if c.dryRun {
		c.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	namespace := k8s.TeamIDToNamespace(teamID)
	releaseName := jupyterReleaseName(namespace)
	if err := helm.Uninstall(releaseName, namespace); err != nil {
		return err
	}

	if err := c.repo.AppDelete(ctx, teamID, gensql.ChartTypeJupyterhub); err != nil {
		return err
	}

	return nil
}

// jupyterReleaseName creates a unique release name based on namespace name.
// The release name must be unique across namespaces as the helm chart creates a clusterrole
// for each jupyterhub with the same name as the release name
func jupyterReleaseName(namespace string) string {
	return fmt.Sprintf("%v-%v", string(gensql.ChartTypeJupyterhub), namespace)
}

func jupyterMergeValues(team gensql.TeamGetRow, configurableValues JupyterConfigurableValues) jupyterValues {
	var profileList string
	if configurableValues.ImageName != "" {
		profileList = fmt.Sprintf(`{"display_name":"Custom image","description":"Custom image for team %v","kubespawner_override":{"image":"%v:%v"}}`,
			configurableValues.Slug, configurableValues.ImageName, configurableValues.ImageTag)
	}

	var allowList string
	if len(configurableValues.AllowList) > 0 {
		allowList = fmt.Sprintf(`{"allowlist": "%v"}`, strings.Join(configurableValues.AllowList, `","`))
	}

	return jupyterValues{
		JupyterConfigurableValues: configurableValues,
		CPULimit:                  configurableValues.CPU,
		CPUGuarantee:              configurableValues.CPU,
		MemoryLimit:               configurableValues.Memory,
		MemoryGuarantee:           configurableValues.Memory,
		AdminUsers:                configurableValues.UserIdents,
		AllowedUsers:              configurableValues.UserIdents,
		Hosts:                     fmt.Sprintf(`["%v"]`, team.Slug+".jupyter.knada.io"),
		IngressTLS:                fmt.Sprintf(`[{"hosts":["%v"], "secretName": "%v"}`, team.Slug+".jupyter.knada.io", "jupyterhub-certificate"),
		OAuthCallbackURL:          fmt.Sprintf("https://%v.jupyter.knada.io/hub/oauth_callback", team.Slug),
		KnadaTeamSecret:           fmt.Sprintf("projects/knada-gcp/secrets/%v", team.ID),
		ProfileList:               profileList,
		ExtraAnnotations:          allowList,
	}
}
