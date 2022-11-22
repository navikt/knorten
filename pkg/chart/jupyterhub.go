package chart

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/helm"
	helmApps "github.com/nais/knorten/pkg/helm/applications"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/nais/knorten/pkg/reflect"
)

type JupyterForm struct {
	TeamID string
	Slug   string
	JupyterValues
}

func (v *JupyterConfigurableValues) MemoryWithoutUnit() string {
	if v.MemoryLimit == "" {
		return ""
	}
	return v.MemoryLimit[:len(v.MemoryLimit)-1]
}

type JupyterConfigurableValues struct {
	CPULimit        string `form:"cpu" helm:"singleuser.cpu.limit"`
	CPUGuarantee    string `form:"cpu" helm:"singleuser.cpu.guarantee"`
	MemoryLimit     string `form:"memory" helm:"singleuser.memory.limit"`
	MemoryGuarantee string `form:"memory" helm:"singleuser.memory.guarantee"`
}

type JupyterValues struct {
	JupyterConfigurableValues

	// Generated config
	AdminUsers       []string `helm:"hub.config.Authenticator.admin_users"`
	AllowedUsers     []string `helm:"hub.config.Authenticator.allowed_users"`
	Hosts            string   `helm:"ingress.hosts"`
	IngressTLS       string   `helm:"ingress.tls"`
	ServiceAccount   string   `helm:"singleuser.serviceAccountName"`
	OAuthCallbackURL string   `helm:"hub.config.AzureAdOAuthenticator.oauth_callback_url"`
	KnadaTeamSecret  string   `helm:"singleuser.extraEnv.KNADA_TEAM_SECRET"`
}

func CreateJupyterhub(c *gin.Context, slug string, repo *database.Repo, helmClient *helm.Client) error {
	var form JupyterForm
	err := c.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return err
	}

	if err := form.ensureValidValues(); err != nil {
		return err
	}

	team, err := repo.TeamGet(c, slug)
	if err != nil {
		return err
	}

	form.Slug = slug
	form.TeamID = team.ID
	form.AdminUsers = team.Users
	form.AllowedUsers = team.Users

	existing, err := repo.TeamValuesGet(c, gensql.ChartTypeJupyterhub, team.ID)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return fmt.Errorf("there already exists a jupyterhub for team '%v'", team.ID)
	}

	addGeneratedJupyterhubConfig(&form)
	return installOrUpdateJupyterhub(c, repo, helmClient, form)
}

func installOrUpdateJupyterhub(c context.Context, repo *database.Repo, helmClient *helm.Client, form JupyterForm) error {
	chartValues, err := reflect.CreateChartValues(form.JupyterValues)
	if err != nil {
		return err
	}

	err = repo.TeamValuesInsert(c, gensql.ChartTypeJupyterhub, chartValues, form.TeamID)
	if err != nil {
		return err
	}

	application := helmApps.NewJupyterhub(form.TeamID, repo)

	// Release name must be unique across namespaces as the helm chart creates a clusterrole
	// for each jupyterhub with the same name as the release name.
	releaseName := fmt.Sprintf("%v-%v", string(gensql.ChartTypeJupyterhub), form.TeamID)
	go helmClient.InstallOrUpgrade(releaseName, k8s.NameToNamespace(form.TeamID), application)
	return nil
}

func UpdateJupyterhub(c *gin.Context, form JupyterForm, repo *database.Repo, helmClient *helm.Client) error {
	team, err := repo.TeamGet(c, form.Slug)
	if err != nil {
		return err
	}

	form.TeamID = team.ID
	form.AdminUsers = team.Users
	form.AllowedUsers = team.Users

	return installOrUpdateJupyterhub(c, repo, helmClient, form)
}

func addGeneratedJupyterhubConfig(values *JupyterForm) {
	values.Hosts = fmt.Sprintf("[\"%v\"]", values.Slug+".jupyter.knada.io")
	values.IngressTLS = fmt.Sprintf("[{\"hosts\":[\"%v\"], \"secretName\": \"%v\"}]", values.Slug+".jupyter.knada.io", "jupyterhub-certificate")
	values.ServiceAccount = values.TeamID
	values.OAuthCallbackURL = fmt.Sprintf("https://%v.jupyter.knada.io/hub/oauth_callback", values.Slug)
	values.KnadaTeamSecret = fmt.Sprintf("projects/%v/secrets/%v", "knada-gcp", values.TeamID)
}
