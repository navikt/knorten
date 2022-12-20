package chart

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	log "github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/crypto"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/google"
	"github.com/nais/knorten/pkg/helm"
	helmApps "github.com/nais/knorten/pkg/helm/applications"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/nais/knorten/pkg/reflect"
)

const (
	sqlProxyHost       = "airflow-sql-proxy"
	dbSecretName       = "airflow-db"
	webserverSecret    = "airflow-webserver"
	resultDBSecretName = "airflow-result-db"
)

type AirflowClient struct {
	repo         *database.Repo
	googleClient *google.Google
	k8sClient    *k8s.Client
	helmClient   *helm.Client
	cryptClient  *crypto.EncrypterDecrypter
}

type AirflowForm struct {
	TeamID string
	Slug   string
	Users  []string

	AirflowValues
}

type AirflowConfigurableValues struct {
	DagRepo       string `form:"repo" binding:"validDagRepo,required" helm:"webserver.extraContainers.[0].args.[0]"`
	DagRepoBranch string `form:"branch" helm:"webserver.extraContainers.[0].args.[1]"`
}

type AirflowValues struct {
	AirflowConfigurableValues

	// Generated config
	WebserverEnv               string `helm:"webserver.env"`
	IngressHosts               string `helm:"ingress.web.hosts"`
	WebserverGitSynkRepo       string `helm:"webserver.extraContainers.[0].args.[0]"`
	WebserverGitSynkRepoBranch string `helm:"webserver.extraContainers.[0].args.[1]"`
	SchedulerGitInitRepo       string `helm:"scheduler.extraInitContainers.[0].args.[0]"`
	SchedulerGitInitRepoBranch string `helm:"scheduler.extraInitContainers.[0].args.[1]"`
	SchedulerGitSynkRepo       string `helm:"scheduler.extraContainers.[0].args.[0]"`
	SchedulerGitSynkRepoBranch string `helm:"scheduler.extraContainers.[0].args.[1]"`
	WorkersGitSynkRepo         string `helm:"workers.extraContainers.[0].args.[0]"`
	WorkersGitSynkRepoBranch   string `helm:"workers.extraContainers.[0].args.[1]"`
	WorkerServiceAccount       string `helm:"workers.serviceAccount.name"`
	ExtraEnvs                  string `helm:"env"`
	MetadataSecretName         string `helm:"data.metadataSecretName"`
	ResultBackendSecretName    string `helm:"data.resultBackendSecretName"`
}

var AirflowValidateDagRepo validator.Func = func(fl validator.FieldLevel) bool {
	repo := fl.Field().String()
	return strings.HasPrefix(repo, "navikt/")
}

func NewAirflowClient(repo *database.Repo, googleClient *google.Google, k8sClient *k8s.Client, helmClient *helm.Client, cryptClient *crypto.EncrypterDecrypter) AirflowClient {
	return AirflowClient{
		repo:         repo,
		googleClient: googleClient,
		k8sClient:    k8sClient,
		helmClient:   helmClient,
		cryptClient:  cryptClient,
	}
}

func (a AirflowClient) Create(c *gin.Context, slug string) error {
	var form AirflowForm
	err := c.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return err
	}

	team, err := a.repo.TeamGet(c, slug)
	if err != nil {
		return err
	}
	if team.PendingAirflowUpgrade {
		log.Info("pending airflow install")
		return nil
	}

	form.Slug = slug
	form.TeamID = team.ID
	form.Users = team.Users

	dbPassword, err := generatePassword(40)
	if err != nil {
		return err
	}

	if err := addGeneratedAirflowConfig(c, dbPassword, &form, a.k8sClient); err != nil {
		return err
	}

	go createAirflowDB(c, form.TeamID, dbPassword, a.googleClient, a.k8sClient)

	go createWebserverSecret(c, form.TeamID, a.k8sClient)

	if err := addAirflowTeamValues(c, a.repo, form); err != nil {
		return err
	}

	InstallOrUpdateAirflow(c, form.TeamID, a.repo, a.helmClient, a.cryptClient)
	return nil
}

func (a AirflowClient) Update(ctx context.Context, form AirflowForm) error {
	setSynkRepoAndBranch(&form)

	team, err := a.repo.TeamGet(ctx, form.Slug)
	if err != nil {
		return err
	}
	if team.PendingAirflowUpgrade {
		log.Info("pending airflow upgrade")
		return nil
	}

	form.TeamID = team.ID
	form.Users = team.Users
	err = setWebserverEnv(&form)
	if err != nil {
		return err
	}

	if err := addAirflowTeamValues(ctx, a.repo, form); err != nil {
		return err
	}

	InstallOrUpdateAirflow(ctx, form.TeamID, a.repo, a.helmClient, a.cryptClient)
	return nil
}

func DeleteAirflow(ctx context.Context, teamSlug string, repo *database.Repo, helmClient *helm.Client, googleClient *google.Google, k8sClient *k8s.Client) error {
	team, err := repo.TeamGet(ctx, teamSlug)
	if err != nil {
		return err
	}

	if err := repo.AppDelete(ctx, team.ID, gensql.ChartTypeAirflow); err != nil {
		return err
	}

	go helmClient.Uninstall(string(gensql.ChartTypeAirflow), k8s.NameToNamespace(team.ID))

	go removeAirflowDB(ctx, team.ID, googleClient, k8sClient)

	return nil
}

func InstallOrUpdateAirflow(ctx context.Context, teamID string, repo *database.Repo, helmClient *helm.Client, cryptor *crypto.EncrypterDecrypter) {
	application := helmApps.NewAirflow(teamID, repo, cryptor)

	go helmClient.InstallOrUpgrade(ctx, string(gensql.ChartTypeAirflow), teamID, application)
}

func addAirflowTeamValues(c context.Context, repo *database.Repo, form AirflowForm) error {
	chartValues, err := reflect.CreateChartValues(form)
	if err != nil {
		return err
	}

	err = repo.TeamValuesInsert(c, gensql.ChartTypeAirflow, chartValues, form.TeamID)
	if err != nil {
		return err
	}

	return nil
}

type airflowEnv struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func addGeneratedAirflowConfig(c *gin.Context, dbPassword string, values *AirflowForm, k8sClient *k8s.Client) error {
	values.IngressHosts = fmt.Sprintf("[{\"name\":\"%v\",\"tls\":{\"enabled\":true,\"secretName\":\"%v\"}}]", values.Slug+".airflow.knada.io", "airflow-certificate")
	values.WorkerServiceAccount = values.TeamID
	setSynkRepoAndBranch(values)
	if err := setUserEnvs(values); err != nil {
		return err
	}

	err := setWebserverEnv(values)
	if err != nil {
		return err
	}

	dbConn := fmt.Sprintf("postgresql://%v:%v@%v:5432/%v?sslmode=disable", values.TeamID, dbPassword, sqlProxyHost, values.TeamID)
	err = k8sClient.CreateOrUpdateSecret(c, dbSecretName, k8s.NameToNamespace(values.TeamID), map[string]string{
		"connection": dbConn,
	})
	if err != nil {
		return err
	}
	values.MetadataSecretName = dbSecretName

	resultDBConn := fmt.Sprintf("db+postgresql://%v:%v@%v:5432/%v?sslmode=disable", values.TeamID, dbPassword, sqlProxyHost, values.TeamID)
	err = k8sClient.CreateOrUpdateSecret(c, resultDBSecretName, k8s.NameToNamespace(values.TeamID), map[string]string{
		"connection": resultDBConn,
	})
	if err != nil {
		return err
	}
	values.ResultBackendSecretName = resultDBSecretName

	return nil
}

func setWebserverEnv(values *AirflowForm) error {
	env := airflowEnv{
		Name:  "AIRFLOW_USERS",
		Value: strings.Join(values.Users, ","),
	}

	envBytes, err := json.Marshal([]airflowEnv{env})
	if err != nil {
		return err
	}

	values.WebserverEnv = string(envBytes)

	return nil
}

func setUserEnvs(values *AirflowForm) error {
	userEnvs := []airflowEnv{
		{
			Name:  "KNADA_TEAM_SECRET",
			Value: fmt.Sprintf("projects/knada-gcp/secrets/%v", values.TeamID),
		},
		{
			Name:  "TEAM",
			Value: values.TeamID,
		},
		{
			Name:  "NAMESPACE",
			Value: k8s.NameToNamespace(values.TeamID),
		},
	}

	envBytes, err := json.Marshal(userEnvs)
	if err != nil {
		return err
	}

	values.ExtraEnvs = string(envBytes)
	return nil
}

func setSynkRepoAndBranch(values *AirflowForm) {
	if values.DagRepoBranch == "" {
		values.DagRepoBranch = "main"
	}

	values.WebserverGitSynkRepo = values.DagRepo
	values.WebserverGitSynkRepoBranch = values.DagRepoBranch
	values.SchedulerGitInitRepo = values.DagRepo
	values.SchedulerGitInitRepoBranch = values.DagRepoBranch
	values.SchedulerGitSynkRepo = values.DagRepo
	values.SchedulerGitSynkRepoBranch = values.DagRepoBranch
	values.WorkersGitSynkRepo = values.DagRepo
	values.WorkersGitSynkRepoBranch = values.DagRepoBranch
}

func createAirflowDB(ctx context.Context, teamID, dbPassword string, googleClient *google.Google, k8sClient *k8s.Client) error {
	dbInstance := airflowDBInstance(teamID)
	if err := googleClient.CreateCloudSQLInstance(ctx, dbInstance); err != nil {
		return err
	}

	if err := googleClient.CreateCloudSQLDatabase(ctx, teamID, dbInstance); err != nil {
		return err
	}

	if err := googleClient.CreateOrUpdateCloudSQLUser(ctx, teamID, dbPassword, dbInstance); err != nil {
		return err
	}

	if err := googleClient.SetSQLClientIAMBinding(ctx, teamID); err != nil {
		return err
	}

	if err := k8sClient.CreateCloudSQLProxy(ctx, sqlProxyHost, teamID, k8s.NameToNamespace(teamID), dbInstance); err != nil {
		return err
	}

	return nil
}

func createWebserverSecret(ctx context.Context, teamID string, k8sClient *k8s.Client) error {
	secretKey, err := generatePassword(32)
	if err != nil {
		return err
	}

	if err := k8sClient.CreateOrUpdateSecret(ctx, webserverSecret, k8s.NameToNamespace(teamID), map[string]string{"webserver-secret-key": secretKey}); err != nil {
		return err
	}

	return nil
}

func removeAirflowDB(ctx context.Context, teamID string, googleClient *google.Google, k8sClient *k8s.Client) error {
	dbInstance := airflowDBInstance(teamID)
	if err := googleClient.DeleteCloudSQLInstance(ctx, dbInstance); err != nil {
		return err
	}

	if err := googleClient.RemoveSQLClientIAMBinding(ctx, teamID); err != nil {
		return err
	}

	namespace := k8s.NameToNamespace(teamID)
	if err := k8sClient.DeleteCloudSQLProxy(ctx, sqlProxyHost, namespace); err != nil {
		return err
	}

	if err := k8sClient.DeleteSecret(ctx, dbSecretName, namespace); err != nil {
		return err
	}

	if err := k8sClient.DeleteSecret(ctx, resultDBSecretName, namespace); err != nil {
		return err
	}

	return nil
}

func generatePassword(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func airflowDBInstance(teamID string) string {
	return "airflow-" + teamID
}
