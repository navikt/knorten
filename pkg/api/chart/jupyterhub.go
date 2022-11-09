package chart

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/helm"
	helmApps "github.com/nais/knorten/pkg/helm/applications"
	"reflect"
)

type JupyterForm struct {
	Namespace string `form:"namespace" binding:"required"`
	JupyterValues
}

type JupyterConfigurableValues struct {
	AdminUsers      []string `form:"users[]" binding:"required" helm:"hub.config.Authenticator.admin_users"`
	AllowedUsers    []string `form:"users[]" binding:"required" helm:"hub.config.Authenticator.allowed_users"`
	CPULimit        string   `form:"cpu" helm:"singleuser.cpu.limit"`
	CPUGuarantee    string   `form:"cpu" helm:"singleuser.cpu.guarantee"`
	MemoryLimit     string   `form:"memory" helm:"singleuser.memory.limit"`
	MemoryGuarantee string   `form:"memory" helm:"singleuser.memory.guarantee"`
}

type JupyterValues struct {
	JupyterConfigurableValues

	// Generated config
	ProxyToken       string `helm:"proxy.secretToken"`
	Hosts            string `helm:"ingress.hosts"`
	IngressTLS       string `helm:"ingress.tls"`
	ServiceAccount   string `helm:"singleuser.serviceAccountName"`
	OAuthCallbackURL string `helm:"hub.config.AzureAdOAuthenticator.oauth_callback_url"`
	KnadaTeamSecret  string `helm:"singleuser.extraEnv.KNADA_TEAM_SECRET"`
}

func CreateJupyterhub(c *gin.Context, repo *database.Repo, helmClient *helm.Client) error {
	var form JupyterForm
	err := c.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return err
	}

	existing, err := repo.TeamValuesGet(c, gensql.ChartTypeJupyterhub, form.Namespace)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return fmt.Errorf("there already exists a jupyterhub for namespace %v", form.Namespace)
	}

	addGeneratedJupyterhubConfig(&form)
	return installOrUpdateJupyterhub(c, repo, helmClient, form)
}

func installOrUpdateJupyterhub(c *gin.Context, repo *database.Repo, helmClient *helm.Client, form JupyterForm) error {
	values := reflect.ValueOf(form.JupyterValues)
	fields := reflect.VisibleFields(reflect.TypeOf(form.JupyterValues))
	chartValues, err := createChartValues(values, fields)
	if err != nil {
		return err
	}

	err = repo.ApplicationCreate(c, gensql.ChartTypeJupyterhub, chartValues, form.Namespace, form.AllowedUsers)
	if err != nil {
		return err
	}

	jupyterhub := helmApps.NewJupyterhub(form.Namespace, repo)
	_, err = jupyterhub.Chart(c)
	if err != nil {
		return err
	}

	go helmClient.InstallOrUpgrade(c, string(gensql.ChartTypeJupyterhub), form.Namespace, jupyterhub)
	return nil
}

func UpdateJupyterhub(c *gin.Context, repo *database.Repo, helmClient *helm.Client) error {
	var form JupyterForm
	err := c.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return err
	}

	return installOrUpdateJupyterhub(c, repo, helmClient, form)
}

func addGeneratedJupyterhubConfig(values *JupyterForm) {
	values.ProxyToken = generateSecureToken(64)
	values.Hosts = fmt.Sprintf("[\"%v\"]", values.Namespace+".jupyter.knada.io")
	values.IngressTLS = fmt.Sprintf("[{\"hosts\":[\"%v\"], \"secretName\": \"%v\"}]", values.Namespace+".jupyter.knada.io", "jupyterhub-certificate")
	values.ServiceAccount = values.Namespace
	values.OAuthCallbackURL = fmt.Sprintf("https://%v.jupyter.knada.io/hub/oauth_callback", values.Namespace)
	values.KnadaTeamSecret = fmt.Sprintf("projects/%v/secrets/%v", "knada-gcp", "team-"+values.Namespace)
}
