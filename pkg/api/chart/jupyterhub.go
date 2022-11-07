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
	"strings"
)

type JupyterForm struct {
	Namespace string `form:"namespace" binding:"required"`
	JupyterValues
}

type JupyterValues struct {
	database.JupyterConfigurableValues

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
	chartValues := map[string]string{}
	values := reflect.ValueOf(form.JupyterValues)
	fields := reflect.VisibleFields(reflect.TypeOf(form.JupyterValues))

	for _, field := range fields {
		tag := field.Tag.Get("helm")
		if tag == "" {
			continue
		}
		value := values.FieldByName(field.Name)
		valueAsString := ""

		switch value.Type().Kind() {
		case reflect.String:
			valueAsString = value.String()
		case reflect.Slice:
			parts, ok := value.Interface().([]string)
			if !ok {
				return fmt.Errorf("unable to parse reflect slice: %v", value)
			}

			quotified := make([]string, len(parts))
			for i, v := range parts {
				quotified[i] = fmt.Sprintf("%q", v)
			}
			valueAsString = fmt.Sprintf("[%v]", strings.Join(quotified, ", "))
		default:
			return fmt.Errorf("helm value must be string or slice of strings, unable to parse helm value: %v", value)

		}

		if valueAsString != "" {
			chartValues[tag] = valueAsString
		}
	}

	err := repo.ApplicationCreate(c, gensql.ChartTypeJupyterhub, chartValues, form.Namespace, form.AllowedUsers)
	if err != nil {
		return err
	}

	jupyterhub := helmApps.NewJupyterhub(form.Namespace, repo)
	_, err = jupyterhub.Chart(c)
	if err != nil {
		return err
	}

	// go helmClient.InstallOrUpgrade(c, string(gensql.ChartTypeJupyterhub), form.Namespace, jupyterhub)
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
