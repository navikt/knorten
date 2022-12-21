package chart

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"

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
	log          *logrus.Entry
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

func NewAirflowClient(repo *database.Repo, googleClient *google.Google, k8sClient *k8s.Client, helmClient *helm.Client, cryptClient *crypto.EncrypterDecrypter, log *logrus.Entry) AirflowClient {
	return AirflowClient{
		repo:         repo,
		googleClient: googleClient,
		k8sClient:    k8sClient,
		helmClient:   helmClient,
		cryptClient:  cryptClient,
		log:          log.WithField("chart", "airflow"),
	}
}

func (a AirflowClient) Create(ctx *gin.Context, slug string) error {
	var form AirflowForm
	err := ctx.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return err
	}

	team, err := a.repo.TeamGet(ctx, slug)
	if err != nil {
		return err
	}

	if team.PendingAirflowUpgrade {
		a.log.Info("pending airflow install")
		return nil
	}

	form.Slug = slug
	form.TeamID = team.ID
	form.Users = team.Users

	dbPassword, err := generatePassword(40)
	if err != nil {
		return err
	}

	if err := a.addGeneratedConfig(ctx, dbPassword, &form); err != nil {
		return err
	}

	go a.createDB(ctx, form.TeamID, dbPassword)

	go a.createWebserverSecret(ctx, form.TeamID)

	if err := a.addAirflowTeamValues(ctx, form); err != nil {
		return err
	}

	application := helmApps.NewAirflow(team.ID, a.repo, a.cryptClient)

	go a.helmClient.InstallOrUpgrade(ctx, string(gensql.ChartTypeAirflow), team.ID, application)

	return nil
}

func (a AirflowClient) Update(ctx context.Context, form AirflowForm) error {
	setSynkRepoAndBranch(&form)

	team, err := a.repo.TeamGet(ctx, form.Slug)
	if err != nil {
		return err
	}
	if team.PendingAirflowUpgrade {
		a.log.Info("pending airflow upgrade")
		return nil
	}

	form.TeamID = team.ID
	form.Users = team.Users
	err = setWebserverEnv(&form)
	if err != nil {
		return err
	}

	if err := a.addAirflowTeamValues(ctx, form); err != nil {
		return err
	}

	application := helmApps.NewAirflow(team.ID, a.repo, a.cryptClient)

	go a.helmClient.InstallOrUpgrade(ctx, string(gensql.ChartTypeAirflow), team.ID, application)

	return nil
}

func (a AirflowClient) Delete(ctx context.Context, teamSlug string) error {
	team, err := a.repo.TeamGet(ctx, teamSlug)
	if err != nil {
		return err
	}

	if team.PendingAirflowUpgrade {
		a.log.Info("pending airflow upgrade")
		return nil
	}

	if err := a.repo.AppDelete(ctx, team.ID, gensql.ChartTypeAirflow); err != nil {
		return err
	}

	go a.helmClient.Uninstall(string(gensql.ChartTypeAirflow), k8s.NameToNamespace(team.ID))

	go a.deleteDB(ctx, team.ID)

	return nil
}

func (a AirflowClient) addAirflowTeamValues(ctx context.Context, form AirflowForm) error {
	chartValues, err := reflect.CreateChartValues(form)
	if err != nil {
		return err
	}

	err = a.repo.TeamValuesInsert(ctx, gensql.ChartTypeAirflow, chartValues, form.TeamID)
	if err != nil {
		return err
	}

	return nil
}

func (a AirflowClient) addGeneratedConfig(ctx context.Context, dbPassword string, values *AirflowForm) error {
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
	err = a.k8sClient.CreateOrUpdateSecret(ctx, dbSecretName, k8s.NameToNamespace(values.TeamID), map[string]string{
		"connection": dbConn,
	})
	if err != nil {
		return err
	}
	values.MetadataSecretName = dbSecretName

	resultDBConn := fmt.Sprintf("db+postgresql://%v:%v@%v:5432/%v?sslmode=disable", values.TeamID, dbPassword, sqlProxyHost, values.TeamID)
	err = a.k8sClient.CreateOrUpdateSecret(ctx, resultDBSecretName, k8s.NameToNamespace(values.TeamID), map[string]string{
		"connection": resultDBConn,
	})
	if err != nil {
		return err
	}
	values.ResultBackendSecretName = resultDBSecretName

	return nil
}

type airflowEnv struct {
	Name  string `json:"name"`
	Value string `json:"value"`
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

func (a AirflowClient) createDB(ctx context.Context, teamID, dbPassword string) {
	dbInstance := CreateAirflowDBInstanceName(teamID)
	if err := a.googleClient.CreateCloudSQLInstance(ctx, dbInstance); err != nil {
		a.log.WithError(err).Errorf("error while creating dbInstance %v for %v", dbInstance, teamID)
		return
	}

	if err := a.googleClient.CreateCloudSQLDatabase(ctx, teamID, dbInstance); err != nil {
		a.log.WithError(err).Errorf("error while creating dbInstance %v for %v", dbInstance, teamID)
		return
	}

	if err := a.googleClient.CreateOrUpdateCloudSQLUser(ctx, teamID, dbPassword, dbInstance); err != nil {
		a.log.WithError(err).Errorf("error while creating dbInstance %v for %v", dbInstance, teamID)
		return
	}

	if err := a.googleClient.SetSQLClientIAMBinding(ctx, teamID); err != nil {
		a.log.WithError(err).Errorf("error while creating dbInstance %v for %v", dbInstance, teamID)
		return
	}

	if err := a.k8sClient.CreateCloudSQLProxy(ctx, sqlProxyHost, teamID, k8s.NameToNamespace(teamID), dbInstance); err != nil {
		a.log.WithError(err).Errorf("error while creating dbInstance %v for %v", dbInstance, teamID)
		return
	}
}

func (a AirflowClient) createWebserverSecret(ctx context.Context, teamID string) {
	secretKey, err := generatePassword(32)
	if err != nil {
		a.log.WithError(err).Errorf("error while generating password for %v", teamID)
		return
	}

	if err := a.k8sClient.CreateOrUpdateSecret(ctx, webserverSecret, k8s.NameToNamespace(teamID), map[string]string{"webserver-secret-key": secretKey}); err != nil {
		a.log.WithError(err).Errorf("error while setting secret %v for %v", webserverSecret, teamID)
		return
	}
}

func (a AirflowClient) deleteDB(ctx context.Context, teamID string) {
	dbInstance := CreateAirflowDBInstanceName(teamID)
	if err := a.googleClient.DeleteCloudSQLInstance(ctx, dbInstance); err != nil {
		a.log.WithError(err).Errorf("error while deleting dbInstace %v for %v", dbInstance, teamID)
		return
	}

	if err := a.googleClient.RemoveSQLClientIAMBinding(ctx, teamID); err != nil {
		a.log.WithError(err).Errorf("error while deleting dbInstace %v for %v", dbInstance, teamID)
		return
	}

	namespace := k8s.NameToNamespace(teamID)
	if err := a.k8sClient.DeleteCloudSQLProxy(ctx, sqlProxyHost, namespace); err != nil {
		a.log.WithError(err).Errorf("error while deleting dbInstace %v for %v", dbInstance, teamID)
		return
	}

	if err := a.k8sClient.DeleteSecret(ctx, dbSecretName, namespace); err != nil {
		a.log.WithError(err).Errorf("error while deleting dbInstace %v for %v", dbInstance, teamID)
		return
	}

	if err := a.k8sClient.DeleteSecret(ctx, resultDBSecretName, namespace); err != nil {
		a.log.WithError(err).Errorf("error while deleting dbInstace %v for %v", dbInstance, teamID)
		return
	}
}

func generatePassword(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func CreateAirflowDBInstanceName(teamID string) string {
	return "airflow-" + teamID
}
