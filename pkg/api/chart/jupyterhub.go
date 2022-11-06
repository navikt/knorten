package chart

import (
	"fmt"
	"reflect"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/helm"
	helmApps "github.com/nais/knorten/pkg/helm/applications"
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

func CreateJupyterhub(c *gin.Context, repo *database.Repo, helmClient *helm.Client, chartType gensql.ChartType) error {
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

	formValues := reflect.ValueOf(form.JupyterValues)
	formFields := reflect.VisibleFields(reflect.TypeOf(form.JupyterValues))
	chartValues := map[string]string{}
	for _, field := range formFields {
		value := formValues.FieldByName(field.Name)
		valueString, err := reflectValueToString(value)
		if err != nil {
			return err
		}

		chartValues[field.Tag.Get("helm")] = valueString
	}

	err = repo.ApplicationCreate(c, chartType, chartValues, form.Namespace, form.AllowedUsers)
	if err != nil {
		return err
	}

	jupyterhub := helmApps.NewJupyterhub(form.Namespace, repo)
	_, err = jupyterhub.Chart(c)
	if err != nil {
		return err
	}

	// debugs
	//jsonBytes, err := json.Marshal(chart.Values)
	//if err != nil {
	//	return err
	//}
	//
	//err = os.WriteFile("out.json", jsonBytes, fs.ModeAppend)
	//if err != nil {
	//	fmt.Println(err)
	//}

	go helmClient.InstallOrUpgrade(c, string(chartType), form.Namespace, jupyterhub)

	return nil
}

func addGeneratedJupyterhubConfig(values *JupyterForm) {
	values.ProxyToken = generateSecureToken(64)
	values.Hosts = fmt.Sprintf("[\"%v\"]", values.Namespace+".jupyter.knada.io")
	values.IngressTLS = fmt.Sprintf("[{\"hosts\":[\"%v\"], \"secretName\": \"%v\"}]", values.Namespace+".jupyter.knada.io", "jupyterhub-certificate")
	values.ServiceAccount = values.Namespace
	values.OAuthCallbackURL = fmt.Sprintf("https://%v.jupyter.knada.io/hub/oauth_callback", values.Namespace)
	values.KnadaTeamSecret = fmt.Sprintf("projects/%v/secrets/%v", "knada-gcp", "team-"+values.Namespace)
}
