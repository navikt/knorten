package chart

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/google"
	"github.com/nais/knorten/pkg/helm"
	helmApps "github.com/nais/knorten/pkg/helm/applications"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/nais/knorten/pkg/reflect"
	"strings"
)

type AirflowForm struct {
	Namespace string `form:"namespace"`
	Users     []string

	AirflowValues
}

type AirflowConfigurableValues struct {
	DagRepo       string `form:"repo" binding:"required" helm:"dags.gitSync.repo"`
	DagRepoBranch string `form:"branch" helm:"dags.gitSync.branch"`
}

type AirflowValues struct {
	AirflowConfigurableValues

	// Generated config
	WebserverEnv               string `helm:"webserver.env"`
	WebserverSecretKey         string `helm:"webserverSecretKey"`
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
	DBUser                     string `helm:"data.metadataConnection.user"`
	DBPassword                 string `helm:"data.metadataConnection.pass"`
	DBHost                     string `helm:"data.metadataConnection.host"`
	DBName                     string `helm:"data.metadataConnection.db"`
}

func installOrUpdateAirflow(ctx context.Context, form AirflowForm, repo *database.Repo, helmClient *helm.Client) error {
	chartValues, err := reflect.CreateChartValues(form)
	if err != nil {
		return err
	}

	err = repo.ServiceCreate(ctx, gensql.ChartTypeAirflow, chartValues, form.Namespace)
	if err != nil {
		return err
	}

	application := helmApps.NewAirflow(form.Namespace, repo)

	go helmClient.InstallOrUpgrade(string(gensql.ChartTypeAirflow), form.Namespace, application)

	return nil
}

func CreateAirflow(c *gin.Context, teamName string, repo *database.Repo, googleClient *google.Google, k8sClient *k8s.Client, helmClient *helm.Client) error {
	var form AirflowForm
	err := c.ShouldBindWith(&form, binding.Form)
	if err != nil {
		return err
	}

	form.Namespace = teamName

	team, err := repo.TeamGet(c, form.Namespace)
	if err != nil {
		return err
	}
	form.Users = team.Users

	if err := addGeneratedAirflowConfig(&form); err != nil {
		return err
	}

	go createAirflowDB(c, teamName, googleClient, k8sClient, &form)

	return installOrUpdateAirflow(c, form, repo, helmClient)
}

func UpdateAirflow(ctx context.Context, form AirflowForm, repo *database.Repo, helmClient *helm.Client) error {
	setSynkRepoAndBranch(&form)

	team, err := repo.TeamGet(ctx, form.Namespace)
	if err != nil {
		return err
	}

	form.Users = team.Users
	err = setWebserverEnv(&form)
	if err != nil {
		return err
	}

	return installOrUpdateAirflow(ctx, form, repo, helmClient)
}

type airflowEnv struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func addGeneratedAirflowConfig(values *AirflowForm) error {
	values.WebserverSecretKey = generateSecureToken(64)
	values.IngressHosts = fmt.Sprintf("[{\"name\":\"%v\",\"tls\":{\"enabled\":true,\"secretName\":\"%v\"}}]", values.Namespace+".airflow.knada.io", "airflow-certificate")
	values.WorkerServiceAccount = values.Namespace
	setSynkRepoAndBranch(values)
	if err := setUserEnvs(values); err != nil {
		return err
	}

	dbPassword, err := generatePassword(40)
	if err != nil {
		return err
	}

	err = setWebserverEnv(values)
	if err != nil {
		return err
	}

	values.DBHost = "airflow-sql-proxy"
	values.DBName = values.Namespace
	values.DBUser = values.Namespace
	values.DBPassword = dbPassword

	return nil
}

func setWebserverEnv(values *AirflowForm) error {
	env := airflowEnv{
		Name:  "AIRFLOW_USERS",
		Value: strings.Join(values.Users, ","),
	}

	envBytes, err := json.Marshal(env)
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
			Value: fmt.Sprintf("projects/%v/secrets/%v", "knada-gcp", values.Namespace),
		},
		{
			Name:  "TEAM",
			Value: values.Namespace,
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

func createAirflowDB(ctx context.Context, team string, googleClient *google.Google, k8sClient *k8s.Client, form *AirflowForm) error {
	dbInstance := "airflow-" + team
	if err := googleClient.CreateCloudSQLInstance(ctx, dbInstance); err != nil {
		return err
	}

	if err := googleClient.CreateCloudSQLDatabase(ctx, team, dbInstance); err != nil {
		return err
	}

	if err := googleClient.CreateCloudSQLUser(ctx, team, form.DBPassword, dbInstance); err != nil {
		return err
	}

	if err := googleClient.CreateSQLClientIAMBinding(ctx, team); err != nil {
		return err
	}

	if err := k8sClient.CreateCloudSQLProxy(ctx, form.DBHost, team, dbInstance); err != nil {
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
