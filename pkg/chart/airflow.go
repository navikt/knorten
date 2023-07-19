package chart

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/helm"
	helmApps "github.com/nais/knorten/pkg/helm/applications"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/nais/knorten/pkg/reflect"
)

const (
	sqlProxyHost                 = "airflow-sql-proxy"
	k8sAirflowDatabaseSecretName = "airflow-db"
	webserverSecret              = "airflow-webserver"
)

type AirflowConfigurableValues struct {
	Slug           string
	DagRepo        string `helm:"webserver.extraContainers.[0].args.[0]"`
	DagRepoBranch  string `helm:"webserver.extraContainers.[0].args.[1]"`
	ApiAccess      bool
	RestrictEgress bool
}

type AirflowValues struct {
	// User-configurable values
	AirflowConfigurableValues

	// Generated config
	WebserverEnv               string `helm:"webserver.env"`
	IngressHosts               string `helm:"ingress.web.hosts"`
	WebserverServiceAccount    string `helm:"webserver.serviceAccount.name"`
	WebserverGitSynkRepo       string `helm:"webserver.extraContainers.[0].args.[0]"`
	WebserverGitSynkRepoBranch string `helm:"webserver.extraContainers.[0].args.[1]"`
	SchedulerGitInitRepo       string `helm:"scheduler.extraInitContainers.[0].args.[0]"`
	SchedulerGitInitRepoBranch string `helm:"scheduler.extraInitContainers.[0].args.[1]"`
	SchedulerGitSynkRepo       string `helm:"scheduler.extraContainers.[0].args.[0]"`
	SchedulerGitSynkRepoBranch string `helm:"scheduler.extraContainers.[0].args.[1]"`
	WorkersGitSynkRepo         string `helm:"workers.extraInitContainers.[0].args.[0]"`
	WorkersGitSynkRepoBranch   string `helm:"workers.extraInitContainers.[0].args.[1]"`
	WorkerServiceAccount       string `helm:"workers.serviceAccount.name"`
	ExtraEnvs                  string `helm:"env"`
	MetadataSecretName         string `helm:"data.metadataSecretName"`
	ResultBackendSecretName    string `helm:"data.resultBackendSecretName"`
	FernetKey                  string `helm:"fernetKey"`
}

func (c Client) syncAirflow(ctx context.Context, configurableValues AirflowConfigurableValues) error {
	team, err := c.repo.TeamGet(ctx, configurableValues.Slug)
	if err != nil {
		return err
	}

	if team.PendingAirflowUpgrade {
		c.log.Info("pending airflow install")
		return nil
	}

	if err := c.restrictAirflowEgress(ctx, configurableValues.RestrictEgress, team.ID); err != nil {
		return err
	}

	if err := c.repo.TeamSetApiAccess(ctx, team.ID, configurableValues.ApiAccess); err != nil {
		return err
	}

	dbPassword, err := generatePassword(40)
	if err != nil {
		return err
	}

	namespace := k8s.TeamIDToNamespace(team.ID)
	secretStringData := map[string]string{
		"connection": fmt.Sprintf("postgresql://%v:%v@%v:5432/%v?sslmode=disable", team.ID, dbPassword, cloudSQLProxyName, team.ID),
	}
	if err := c.createOrUpdateSecret(ctx, k8sAirflowDatabaseSecretName, namespace, secretStringData); err != nil {
		return err
	}

	bucketName := fmt.Sprintf("airflow-logs-%v", team.ID)

	values, err := mergeAirflowValues(bucketName, team, configurableValues)
	if err != nil {
		return err
	}

	err = c.createAirflowDatabase(ctx, team.ID, dbPassword)
	if err != nil {
		return err
	}

	err = c.createLogBucketForAirflow(ctx, team.ID, bucketName)
	if err != nil {
		return err
	}

	if err := c.createAirflowWebserverSecret(ctx, team.ID); err != nil {
		return err
	}

	chartValues, err := reflect.CreateChartValues(values)
	if err != nil {
		return err
	}

	err = c.repo.TeamValuesInsert(ctx, gensql.ChartTypeAirflow, chartValues, team.ID)
	if err != nil {
		return err
	}

	application := helmApps.NewAirflow(team.ID, c.repo, c.chartVersionAirflow)
	charty, err := application.Chart(ctx)
	if err != nil {
		return err
	}

	return helm.InstallOrUpgrade(string(gensql.ChartTypeAirflow), c.chartVersionJupyter, namespace, charty.Values)
}

func (c Client) deleteAirflow(ctx context.Context, teamID string) error {
	if c.dryRun {
		c.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	namespace := k8s.TeamIDToNamespace(teamID)

	if err := helm.Uninstall(string(gensql.ChartTypeAirflow), namespace); err != nil {
		return err
	}

	if err := c.deleteCloudSQLProxy(ctx, namespace); err != nil {
		return err
	}

	if err := c.deleteSecret(ctx, k8sAirflowDatabaseSecretName, namespace); err != nil {
		return err
	}

	if err := removeSQLClientIAMBinding(ctx, c.gcpProject, teamID); err != nil {
		return err
	}

	instanceName := createAirflowcloudSQLInstanceName(teamID)
	if err := deleteCloudSQLInstance(ctx, instanceName, c.gcpProject); err != nil {
		return err
	}

	if err := c.repo.AppDelete(ctx, teamID, gensql.ChartTypeAirflow); err != nil {
		return err
	}

	return nil
}

func mergeAirflowValues(bucketName string, team gensql.TeamGetRow, configurableValues AirflowConfigurableValues) (AirflowValues, error) {
	fernetKey, err := generateFernetKey()
	if err != nil {
		return AirflowValues{}, err
	}

	extraEnvs, err := generateAirflowExtraEnvs(bucketName, team.ID)
	if err != nil {
		return AirflowValues{}, err
	}

	webserverEnv, err := generateAirflowWebServerEnvs(team.Users, configurableValues.ApiAccess)
	if err != nil {
		return AirflowValues{}, err
	}

	return AirflowValues{
		IngressHosts:               fmt.Sprintf("[{\"name\":\"%v\",\"tls\":{\"enabled\":true,\"secretName\":\"%v\"}}]", configurableValues.Slug+".airflow.knada.io", "airflow-certificate"),
		WebserverServiceAccount:    team.ID,
		WorkerServiceAccount:       team.ID,
		MetadataSecretName:         k8sAirflowDatabaseSecretName,
		ResultBackendSecretName:    k8sAirflowDatabaseSecretName,
		FernetKey:                  fernetKey,
		WebserverGitSynkRepo:       configurableValues.DagRepo,
		WebserverGitSynkRepoBranch: configurableValues.DagRepoBranch,
		SchedulerGitInitRepo:       configurableValues.DagRepo,
		SchedulerGitInitRepoBranch: configurableValues.DagRepoBranch,
		SchedulerGitSynkRepo:       configurableValues.DagRepo,
		SchedulerGitSynkRepoBranch: configurableValues.DagRepoBranch,
		WorkersGitSynkRepo:         configurableValues.DagRepo,
		WorkersGitSynkRepoBranch:   configurableValues.DagRepoBranch,
		ExtraEnvs:                  extraEnvs,
		WebserverEnv:               webserverEnv,
	}, nil
}

type airflowEnv struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func generateAirflowWebServerEnvs(users []string, apiAccess bool) (string, error) {
	envs := []airflowEnv{
		{
			Name:  "AIRFLOW_USERS",
			Value: strings.Join(users, ","),
		},
	}

	if apiAccess {
		// TODO: Sjekk om dette faktisk er nødvendig, jeg trodde basic_auth stod i veien for måten vi løste det på
		envs = append(envs, airflowEnv{
			Name:  "AIRFLOW__API__AUTH_BACKENDS",
			Value: "airflow.api.auth.backend.session,airflow.api.auth.backend.basic_auth",
		})
	}

	envBytes, err := json.Marshal(envs)
	if err != nil {
		return "", err
	}

	return string(envBytes), nil
}

func generateAirflowExtraEnvs(bucketName, teamID string) (string, error) {
	userEnvs := []airflowEnv{
		{
			Name:  "KNADA_TEAM_SECRET",
			Value: fmt.Sprintf("projects/knada-gcp/secrets/%v", teamID),
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
			Value: fmt.Sprintf("gs://%v", bucketName),
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

func (c Client) restrictAirflowEgress(ctx context.Context, restrictAirflowEgress bool, teamID string) error {
	namespace := k8s.TeamIDToNamespace(teamID)

	if err := c.defaultEgressNetpolSync(ctx, namespace, restrictAirflowEgress); err != nil {
		return err
	}

	if err := c.repo.TeamSetRestrictAirflowEgress(ctx, teamID, restrictAirflowEgress); err != nil {
		return err
	}

	return nil
}

func (c Client) createAirflowDatabase(ctx context.Context, teamID, dbPassword string) error {
	if c.dryRun {
		c.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	dbInstance := createAirflowcloudSQLInstanceName(teamID)
	if err := createCloudSQLInstance(ctx, dbInstance, c.gcpProject, c.gcpRegion); err != nil {
		return err
	}

	if err := createCloudSQLDatabase(ctx, teamID, dbInstance, c.gcpProject); err != nil {
		return err
	}

	if err := createOrUpdateCloudSQLUser(ctx, teamID, dbPassword, dbInstance, c.gcpProject); err != nil {
		return err
	}

	if err := setSQLClientIAMBinding(ctx, teamID, c.gcpProject); err != nil {
		return err
	}

	return c.createCloudSQLProxy(ctx, sqlProxyHost, teamID, k8s.TeamIDToNamespace(teamID), dbInstance)
}

func (c Client) createLogBucketForAirflow(ctx context.Context, teamID, bucketName string) error {
	if c.dryRun {
		c.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	if err := createBucket(ctx, teamID, bucketName, c.gcpProject, c.gcpRegion); err != nil {
		return err
	}

	if err := createServiceAccountObjectAdminBinding(ctx, teamID, bucketName, c.gcpProject); err != nil {
		return err
	}

	return nil
}

func (c Client) createAirflowWebserverSecret(ctx context.Context, teamID string) error {
	secretKey, err := generatePassword(32)
	if err != nil {
		return err
	}

	stringData := map[string]string{"webserver-secret-key": secretKey}
	return c.createOrUpdateSecret(ctx, webserverSecret, k8s.TeamIDToNamespace(teamID), stringData)
}
func createAirflowcloudSQLInstanceName(teamID string) string {
	return "airflow-" + teamID
}

func generatePassword(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func generateFernetKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(b), nil
}
