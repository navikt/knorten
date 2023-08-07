package chart

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/helm"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/nais/knorten/pkg/reflect"
)

const (
	k8sAirflowFernetKeySecretName = "airflow-fernet-key"
	k8sAirflowDatabaseSecretName  = "airflow-db"
	k8sAirflowWebserverSecretName = "airflow-webserver"
	teamValueKeyDatabasePassword  = "databasePassword,omit"
	teamValueKeyFernetKey         = "fernetKey,omit"
	teamValueKeyWebserverSecret   = "webserverSecretKey,omit"
)

type AirflowConfigurableValues struct {
	TeamID         string
	DagRepo        string `helm:"webserver.extraContainers.[0].args.[0]"`
	DagRepoBranch  string `helm:"webserver.extraContainers.[0].args.[1]"`
	RestrictEgress bool
}

type AirflowValues struct {
	// User-configurable values
	AirflowConfigurableValues

	// Manually save to database

	FernetKey          string // Knorten sets Helm value pointing to k8s secret
	PostgresPassword   string // Knorten sets Helm value pointing to k8s secret
	WebserverSecretKey string // Knorten sets Helm value pointing to k8s secret

	// Generated Helm config

	ExtraEnvs                  string `helm:"env"`
	IngressHosts               string `helm:"ingress.web.hosts"`
	SchedulerGitInitRepo       string `helm:"scheduler.extraInitContainers.[0].args.[0]"`
	SchedulerGitInitRepoBranch string `helm:"scheduler.extraInitContainers.[0].args.[1]"`
	SchedulerGitSynkRepo       string `helm:"scheduler.extraContainers.[0].args.[0]"`
	SchedulerGitSynkRepoBranch string `helm:"scheduler.extraContainers.[0].args.[1]"`
	WebserverEnv               string `helm:"webserver.env"`
	WebserverGitSynkRepo       string `helm:"webserver.extraContainers.[0].args.[0]"`
	WebserverGitSynkRepoBranch string `helm:"webserver.extraContainers.[0].args.[1]"`
	WebserverServiceAccount    string `helm:"webserver.serviceAccount.name"`
	WorkerServiceAccount       string `helm:"workers.serviceAccount.name"`
	WorkersGitSynkRepo         string `helm:"workers.extraInitContainers.[0].args.[0]"`
	WorkersGitSynkRepoBranch   string `helm:"workers.extraInitContainers.[0].args.[1]"`
}

func (c Client) syncAirflow(ctx context.Context, configurableValues AirflowConfigurableValues) error {
	team, err := c.repo.TeamGet(ctx, configurableValues.TeamID)
	if err != nil {
		return err
	}

	values, err := c.mergeAirflowValues(ctx, team, configurableValues)
	if err != nil {
		return err
	}

	helmChartValues, err := reflect.CreateChartValues(values)
	if err != nil {
		return err
	}

	// First we save all variables to the database, then we apply them to the cluster.
	if err := c.repo.HelmChartValuesInsert(ctx, gensql.ChartTypeAirflow, helmChartValues, team.ID); err != nil {
		return err
	}

	if err := c.repo.TeamValueInsert(ctx, gensql.ChartTypeAirflow, teamValueKeyDatabasePassword, values.PostgresPassword, team.ID); err != nil {
		return err
	}

	if err := c.repo.TeamValueInsert(ctx, gensql.ChartTypeAirflow, teamValueKeyFernetKey, values.FernetKey, team.ID); err != nil {
		return err
	}

	if err := c.repo.TeamValueInsert(ctx, gensql.ChartTypeAirflow, teamValueKeyWebserverSecret, values.WebserverSecretKey, team.ID); err != nil {
		return err
	}

	if err := c.repo.TeamSetRestrictAirflowEgress(ctx, team.ID, values.RestrictEgress); err != nil {
		return err
	}

	// Apply values to cluster
	namespace := k8s.TeamIDToNamespace(team.ID)

	if err := c.defaultEgressNetpolSync(ctx, namespace, values.RestrictEgress); err != nil {
		return err
	}

	secretStringData := map[string]string{
		"connection": fmt.Sprintf("postgresql://%v:%v@%v:5432/%v?sslmode=disable", team.ID, values.PostgresPassword, cloudSQLProxyName, team.ID),
	}
	if err := c.createOrUpdateSecret(ctx, k8sAirflowDatabaseSecretName, namespace, secretStringData); err != nil {
		return err
	}

	secretStringData = map[string]string{"webserver-secret-key": values.WebserverSecretKey}
	if err := c.createOrUpdateSecret(ctx, k8sAirflowWebserverSecretName, namespace, secretStringData); err != nil {
		return err
	}

	secretStringData = map[string]string{"fernet-key": values.FernetKey}
	if err := c.createOrUpdateSecret(ctx, k8sAirflowFernetKeySecretName, namespace, secretStringData); err != nil {
		return err
	}

	// Apply values to GCP project
	if err := c.createAirflowDatabase(ctx, team.ID, values.PostgresPassword); err != nil {
		return err
	}

	if err := c.createLogBucketForAirflow(ctx, team.ID); err != nil {
		return err
	}

	return helm.InstallOrUpgrade(ctx, c.dryRun, string(gensql.ChartTypeAirflow), namespace, team.ID, "airflow", "apache-airflow", c.chartVersionAirflow, gensql.ChartTypeAirflow, c.repo)
}

func (c Client) deleteAirflow(ctx context.Context, teamID string) error {
	if c.dryRun {
		return nil
	}

	namespace := k8s.TeamIDToNamespace(teamID)

	if err := helm.Uninstall(string(gensql.ChartTypeAirflow), namespace); err != nil {
		return err
	}

	if err := c.deleteCloudSQLProxyFromKubernetes(ctx, namespace); err != nil {
		return err
	}

	if err := c.deleteSecretFromKubernetes(ctx, k8sAirflowDatabaseSecretName, namespace); err != nil {
		return err
	}

	if err := removeSQLClientIAMBinding(c.gcpProject, teamID); err != nil {
		return err
	}

	instanceName := createAirflowcloudSQLInstanceName(teamID)
	if err := deleteCloudSQLInstance(instanceName, c.gcpProject); err != nil {
		return err
	}

	if err := c.repo.AppDelete(ctx, teamID, gensql.ChartTypeAirflow); err != nil {
		return err
	}

	return nil
}

// mergeAirflowValues merges the values from the database with the values from the request, generate the missing values and returns the final values.
func (c Client) mergeAirflowValues(ctx context.Context, team gensql.TeamGetRow, configurableValues AirflowConfigurableValues) (AirflowValues, error) {
	if configurableValues.DagRepo == "" { // only required value
		dagRepo, err := c.repo.TeamValueGet(ctx, "webserver.extraContainers.[0].args.[0]", team.ID)
		if err != nil {
			return AirflowValues{}, err
		}

		configurableValues.DagRepo = dagRepo.Value

		dagRepoBranch, err := c.repo.TeamValueGet(ctx, "webserver.extraContainers.[0].args.[1]", team.ID)
		if err != nil {
			return AirflowValues{}, err
		}

		configurableValues.DagRepoBranch = dagRepoBranch.Value

		configurableValues.RestrictEgress = team.RestrictAirflowEgress
	}

	postgresPassword, err := c.getOrGeneratePassword(ctx, team.ID, teamValueKeyDatabasePassword, generatePassword)
	if err != nil {
		return AirflowValues{}, err
	}

	fernetKey, err := c.getOrGeneratePassword(ctx, team.ID, "fernetKey", generateFernetKey)
	if err != nil {
		return AirflowValues{}, err
	}

	webserverSecretKey, err := c.getOrGeneratePassword(ctx, team.ID, teamValueKeyWebserverSecret, generatePassword)
	if err != nil {
		return AirflowValues{}, err
	}

	extraEnvs, err := c.createAirflowExtraEnvs(team.ID)
	if err != nil {
		return AirflowValues{}, err
	}

	webserverEnv, err := c.createAirflowWebServerEnvs(team.Users)
	if err != nil {
		return AirflowValues{}, err
	}

	return AirflowValues{
		ExtraEnvs:                  extraEnvs,
		FernetKey:                  fernetKey,
		IngressHosts:               fmt.Sprintf(`[{"name":"%v","tls":{"enabled":true,"secretName":"%v"}}]`, team.Slug+".airflow.knada.io", "airflow-certificate"),
		PostgresPassword:           postgresPassword,
		SchedulerGitInitRepo:       configurableValues.DagRepo,
		SchedulerGitInitRepoBranch: configurableValues.DagRepoBranch,
		SchedulerGitSynkRepo:       configurableValues.DagRepo,
		SchedulerGitSynkRepoBranch: configurableValues.DagRepoBranch,
		WebserverEnv:               webserverEnv,
		WebserverGitSynkRepo:       configurableValues.DagRepo,
		WebserverGitSynkRepoBranch: configurableValues.DagRepoBranch,
		WebserverSecretKey:         webserverSecretKey,
		WebserverServiceAccount:    team.ID,
		WorkerServiceAccount:       team.ID,
		WorkersGitSynkRepo:         configurableValues.DagRepo,
		WorkersGitSynkRepoBranch:   configurableValues.DagRepoBranch,
	}, nil
}

type airflowEnv struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (Client) createAirflowWebServerEnvs(users []string) (string, error) {
	envs := []airflowEnv{
		{
			Name:  "AIRFLOW_USERS",
			Value: strings.Join(users, ","),
		},
	}

	envBytes, err := json.Marshal(envs)
	if err != nil {
		return "", err
	}

	return string(envBytes), nil
}

func (c Client) createAirflowExtraEnvs(teamID string) (string, error) {
	userEnvs := []airflowEnv{
		{
			Name:  "KNADA_TEAM_SECRET",
			Value: fmt.Sprintf("projects/%v/secrets/%v", c.gcpProject, teamID),
		},
		{
			Name:  "TEAM",
			Value: teamID,
		},
		{
			Name:  "NAMESPACE",
			Value: k8s.TeamIDToNamespace(teamID),
		},
		{
			Name:  "AIRFLOW__LOGGING__REMOTE_BASE_LOG_FOLDER",
			Value: fmt.Sprintf("gs://%v", createBucketName(teamID)),
		},
		{
			Name:  "AIRFLOW__LOGGING__REMOTE_LOGGING",
			Value: "True",
		},
	}

	envBytes, err := json.Marshal(userEnvs)
	if err != nil {
		return "", err
	}

	return string(envBytes), nil
}

func (c Client) createAirflowDatabase(ctx context.Context, teamID, dbPassword string) error {
	if c.dryRun {
		return nil
	}

	dbInstance := createAirflowcloudSQLInstanceName(teamID)
	if err := createCloudSQLInstance(dbInstance, c.gcpProject, c.gcpRegion); err != nil {
		return err
	}

	if err := createCloudSQLDatabase(teamID, dbInstance, c.gcpProject); err != nil {
		return err
	}

	if err := createOrUpdateCloudSQLUser(teamID, dbPassword, dbInstance, c.gcpProject); err != nil {
		return err
	}

	if err := setSQLClientIAMBinding(teamID, c.gcpProject); err != nil {
		return err
	}

	return c.createCloudSQLProxy(ctx, "airflow-sql-proxy", teamID, k8s.TeamIDToNamespace(teamID), dbInstance)
}

func (c Client) createLogBucketForAirflow(ctx context.Context, teamID string) error {
	if c.dryRun {
		return nil
	}

	bucketName := createBucketName(teamID)
	if err := createBucket(ctx, teamID, bucketName, c.gcpProject, c.gcpRegion); err != nil {
		return err
	}

	if err := createServiceAccountObjectAdminBinding(ctx, teamID, bucketName, c.gcpProject); err != nil {
		return err
	}

	return nil
}

func createAirflowcloudSQLInstanceName(teamID string) string {
	return "airflow-" + teamID
}

func createBucketName(teamID string) string {
	return "airflow-logs-" + teamID
}

func generatePassword() (string, error) {
	b := make([]byte, 40)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}

func generateFernetKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(key), nil
}

func (c Client) getOrGeneratePassword(ctx context.Context, teamID, key string, generator func() (string, error)) (string, error) {
	value, err := c.repo.TeamValueGet(ctx, key, teamID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return generator()
		}
		return "", err
	}

	if value.ChartType == gensql.ChartTypeAirflow && value.Value != "" {
		return value.Value, nil
	}

	return "", fmt.Errorf("a %v exisits for %v, but it's empty or doesn't belong to Airflow", key, teamID)
}
