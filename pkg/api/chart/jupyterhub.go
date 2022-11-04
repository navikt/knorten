package chart

import (
	"fmt"
	"reflect"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
)

type JupyterForm struct {
	Namespace string `form:"namespace" binding:"required"`
	JupyterValues
}

type JupyterValues struct {
	// User config
	AdminUsers      []string `form:"users[]" binding:"required" helm:"hub.config.Authenticator.admin_users"`
	AllowedUsers    []string `form:"users[]" binding:"required" helm:"hub.config.Authenticator.allowed_users"`
	CPULimit        string   `form:"cpu" helm:"singleuser.cpu.limit"`
	CPUGuarantee    string   `form:"cpu" helm:"singleuser.cpu.guarantee"`
	MemoryLimit     string   `form:"memory" helm:"singleuser.memory.limit"`
	MemoryGuarantee string   `form:"memory" helm:"singleuser.memory.guarantee"`

	// Generated config
	ProxyToken       string `helm:"proxy.secretToken"`
	Hosts            string `helm:"ingress.hosts"`
	IngressTLS       string `helm:"ingress.tls"`
	ServiceAccount   string `helm:"singleuser.serviceAccountName"`
	OAuthCallbackURL string `helm:"hub.config.AzureAdOAuthenticator.oauth_callback_url"`
	KnadaTeamSecret  string `helm:"singleuser.extraEnv.KNADA_TEAM_SECRET"`
}

func CreateJupyterhub(c *gin.Context, repo *database.Repo, chartType gensql.ChartType) error {
	var form JupyterForm
	err := c.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return err
	}

	addGeneratedJupyterhubConfig(&form)

	values := reflect.ValueOf(form.JupyterValues)
	fields := reflect.VisibleFields(reflect.TypeOf(form.JupyterValues))
	for _, field := range fields {
		value := values.FieldByName(field.Name)
		valueString, err := reflectValueToString(value)
		if err != nil {
			return err
		}

		err = repo.TeamChartValueInsert(c, field.Tag.Get("helm"), valueString, form.Namespace, chartType)
		if err != nil {
			return err
		}
	}

	for _, user := range form.AllowedUsers {
		err = repo.UserAppInsert(c, user, form.Namespace, chartType)
		if err != nil {
			return err
		}
	}

	return nil
}

func addGeneratedJupyterhubConfig(values *JupyterForm) {
	values.ProxyToken = generateSecureToken(64)
	values.Hosts = fmt.Sprintf("[%v]", values.Namespace+".jupyter.knada.io")
	values.IngressTLS = fmt.Sprintf("[hosts: {hosts: [%v]}, secretName: %v]", values.Namespace+".jupyter.knada.io", "jupyterhub-certificate")
	values.ServiceAccount = values.Namespace
	values.OAuthCallbackURL = fmt.Sprintf("https://%v.jupyter.knada.io/hub/oauth_callback", values.Namespace)
	values.KnadaTeamSecret = fmt.Sprintf("projects/%v/secrets/%v", "knada-gcp", "team-"+values.Namespace)
}
