package chart

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/crypto"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/helm"
	helmApps "github.com/nais/knorten/pkg/helm/applications"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/nais/knorten/pkg/reflect"
	log "github.com/sirupsen/logrus"
)

type JupyterForm struct {
	TeamID    string
	Slug      string
	ImageName string
	ImageTag  string
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
	ImageName       string `form:"imagename" helm:"singleuser.image.name"`
	ImageTag        string `form:"imagetag" helm:"singleuser.image.tag"`
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

func CreateJupyterhub(c *gin.Context, slug string, repo *database.Repo, helmClient *helm.Client, cryptor *crypto.EncrypterDecrypter) error {
	var form JupyterForm
	err := c.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return err
	}

	team, err := repo.TeamGet(c, slug)
	if err != nil {
		return err
	}
	if team.PendingJupyterUpgrade {
		log.Info("pending jupyterhub install")
		return nil
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

	if err := form.ensureValidValues(); err != nil {
		return err
	}

	return UpdateJupyterTeamValuesAndInstall(c, form, repo, helmClient, cryptor)
}

func UpdateJupyterhub(c *gin.Context, form JupyterForm, repo *database.Repo, helmClient *helm.Client, cryptor *crypto.EncrypterDecrypter) error {
	team, err := repo.TeamGet(c, form.Slug)
	if err != nil {
		return err
	}
	if team.PendingJupyterUpgrade {
		log.Info("pending jupyterhub upgrade")
		return nil
	}

	form.TeamID = team.ID
	form.AdminUsers = team.Users
	form.AllowedUsers = team.Users

	if err := form.ensureValidValues(); err != nil {
		return err
	}

	return UpdateJupyterTeamValuesAndInstall(c, form, repo, helmClient, cryptor)
}

func UpdateJupyterTeamValuesAndInstall(c *gin.Context, form JupyterForm, repo *database.Repo, helmClient *helm.Client, cryptor *crypto.EncrypterDecrypter) error {
	if err := storeJupyterTeamValues(c, repo, form); err != nil {
		return err
	}

	InstallOrUpdateJupyterhub(c, form.TeamID, repo, helmClient, cryptor)
	return nil
}

func InstallOrUpdateJupyterhub(ctx context.Context, teamID string, repo *database.Repo, helmClient *helm.Client, cryptor *crypto.EncrypterDecrypter) {
	application := helmApps.NewJupyterhub(teamID, repo, cryptor)

	// Release name must be unique across namespaces as the helm chart creates a clusterrole
	// for each jupyterhub with the same name as the release name.
	releaseName := jupyterReleaseName(k8s.NameToNamespace(teamID))
	go helmClient.InstallOrUpgrade(ctx, releaseName, teamID, application)
}

func DeleteJupyterhub(c context.Context, teamSlug string, repo *database.Repo, helmClient *helm.Client) error {
	team, err := repo.TeamGet(c, teamSlug)
	if err != nil {
		return err
	}

	if err := repo.AppDelete(c, team.ID, gensql.ChartTypeJupyterhub); err != nil {
		return err
	}

	namespace := k8s.NameToNamespace(team.ID)
	releaseName := jupyterReleaseName(namespace)
	go helmClient.Uninstall(releaseName, namespace)

	return nil
}

func storeJupyterTeamValues(c context.Context, repo *database.Repo, form JupyterForm) error {
	chartValues, err := reflect.CreateChartValues(form.JupyterValues)
	if err != nil {
		return err
	}

	err = repo.TeamValuesInsert(c, gensql.ChartTypeJupyterhub, chartValues, form.TeamID)
	if err != nil {
		return err
	}

	return nil
}

func jupyterReleaseName(namespace string) string {
	return fmt.Sprintf("%v-%v", string(gensql.ChartTypeJupyterhub), namespace)
}

func addGeneratedJupyterhubConfig(values *JupyterForm) {
	values.Hosts = fmt.Sprintf("[\"%v\"]", values.Slug+".jupyter.knada.io")
	values.IngressTLS = fmt.Sprintf("[{\"hosts\":[\"%v\"], \"secretName\": \"%v\"}]", values.Slug+".jupyter.knada.io", "jupyterhub-certificate")
	values.ServiceAccount = values.TeamID
	values.OAuthCallbackURL = fmt.Sprintf("https://%v.jupyter.knada.io/hub/oauth_callback", values.Slug)
	values.KnadaTeamSecret = fmt.Sprintf("projects/knada-gcp/secrets/%v", values.TeamID)
}
