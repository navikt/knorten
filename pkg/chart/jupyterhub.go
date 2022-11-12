package chart

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/helm"
	helmApps "github.com/nais/knorten/pkg/helm/applications"
	"github.com/nais/knorten/pkg/reflect"
)

type JupyterForm struct {
	Namespace string `form:"namespace"`
	JupyterValues
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
	ProxyToken       string   `helm:"proxy.secretToken"`
	Hosts            string   `helm:"ingress.hosts"`
	IngressTLS       string   `helm:"ingress.tls"`
	ServiceAccount   string   `helm:"singleuser.serviceAccountName"`
	OAuthCallbackURL string   `helm:"hub.config.AzureAdOAuthenticator.oauth_callback_url"`
	KnadaTeamSecret  string   `helm:"singleuser.extraEnv.KNADA_TEAM_SECRET"`
}

func CreateJupyterhub(c *gin.Context, teamName string, repo *database.Repo, helmClient *helm.Client) error {
	var form JupyterForm
	err := c.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return err
	}

	if err := form.ensureValidValues(); err != nil {
		return err
	}

	form.Namespace = teamName

	team, err := repo.TeamGet(c, form.Namespace)
	if err != nil {
		return err
	}
	form.AdminUsers = team.Users
	form.AllowedUsers = team.Users

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
	chartValues, err := reflect.CreateChartValues(form.JupyterValues)
	if err != nil {
		return err
	}

	err = repo.ServiceCreate(c, gensql.ChartTypeJupyterhub, chartValues, form.Namespace)
	if err != nil {
		return err
	}

	application := helmApps.NewJupyterhub(form.Namespace, repo)
	charty, err := application.Chart(c)
	if err != nil {
		return err
	}

	bytes, err := json.Marshal(charty)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile("out.json", bytes, fs.ModeAppend)
	if err != nil {
		fmt.Println(err)
	}

	// Release name must be unique across namespaces as the helm chart creates a clusterrole
	// for each jupyterhub with the same name as the release name.
	releaseName := fmt.Sprintf("%v-%v", string(gensql.ChartTypeJupyterhub), form.Namespace)
	go helmClient.InstallOrUpgrade(releaseName, form.Namespace, application)
	return nil
}

func UpdateJupyterhub(c *gin.Context, teamName string, repo *database.Repo, helmClient *helm.Client) error {
	var form JupyterForm
	err := c.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return err
	}

	_, err = repo.TeamGet(c, teamName)
	if err != nil {
		return err
	}

	return installOrUpdateJupyterhub(c, repo, helmClient, form)
}

func generateSecureToken(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

func addGeneratedJupyterhubConfig(values *JupyterForm) {
	values.ProxyToken = generateSecureToken(64)
	values.Hosts = fmt.Sprintf("[\"%v\"]", values.Namespace+".jupyter.knada.io")
	values.IngressTLS = fmt.Sprintf("[{\"hosts\":[\"%v\"], \"secretName\": \"%v\"}]", values.Namespace+".jupyter.knada.io", "jupyterhub-certificate")
	values.ServiceAccount = values.Namespace
	values.OAuthCallbackURL = fmt.Sprintf("https://%v.jupyter.knada.io/hub/oauth_callback", values.Namespace)
	values.KnadaTeamSecret = fmt.Sprintf("projects/%v/secrets/%v", "knada-gcp", values.Namespace)
}
