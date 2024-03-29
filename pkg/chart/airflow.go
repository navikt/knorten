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
	"strconv"
	"strings"

	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/database/gensql"
	"github.com/navikt/knorten/pkg/helm"
	"github.com/navikt/knorten/pkg/k8s"
	"github.com/navikt/knorten/pkg/logger"
	"github.com/navikt/knorten/pkg/reflect"
)

const (
	k8sAirflowFernetKeySecretName = "airflow-fernet-key"
	k8sAirflowDatabaseSecretName  = "airflow-db"
	k8sAirflowWebserverSecretName = "airflow-webserver"
	teamValueKeyDatabasePassword  = "databasePassword,omit"
	teamValueKeyFernetKey         = "fernetKey,omit"
	teamValueKeyWebserverSecret   = "webserverSecretKey,omit"
	TeamValueKeyRestrictEgress    = "restrictEgress,omit"
	TeamValueKeyApiAccess         = "apiAccess,omit"
)

type AirflowConfigurableValues struct {
	TeamID         string
	DagRepo        string `helm:"dags.gitSync.repo"`
	DagRepoBranch  string `helm:"dags.gitSync.branch"`
	AirflowImage   string `helm:"images.airflow.repository"`
	AirflowTag     string `helm:"images.airflow.tag"`
	RestrictEgress bool
	ApiAccess      bool
}

type AirflowValues struct {
	// User-configurable values
	AirflowConfigurableValues

	// Manually save to database

	FernetKey          string // Knorten sets Helm value pointing to k8s secret
	PostgresPassword   string // Knorten sets Helm value pointing to k8s secret
	WebserverSecretKey string // Knorten sets Helm value pointing to k8s secret

	// Generated Helm config
	ExtraEnvs               string `helm:"env"`
	WebserverEnv            string `helm:"webserver.env"`
	WebserverServiceAccount string `helm:"webserver.serviceAccount.name"`
	WorkerServiceAccount    string `helm:"workers.serviceAccount.name"`
	WorkerLabels            string `helm:"workers.labels"`
}

func (c Client) syncAirflow(ctx context.Context, configurableValues AirflowConfigurableValues, log logger.Logger) error {
	team, err := c.repo.TeamGet(ctx, configurableValues.TeamID)
	if err != nil {
		log.WithError(err).Infof("getting team %v from database", configurableValues.TeamID)
		return err
	}

	values, err := c.mergeAirflowValues(ctx, team, configurableValues)
	if err != nil {
		log.WithError(err).Info("merging airflow values")
		return err
	}

	helmChartValues, err := reflect.CreateChartValues(values)
	if err != nil {
		log.WithError(err).Info("creating chart values")
		return err
	}

	// First we save all variables to the database, then we apply them to the cluster.
	if err := c.repo.HelmChartValuesInsert(ctx, gensql.ChartTypeAirflow, helmChartValues, team.ID); err != nil {
		log.WithError(err).Info("inserting helm values to database")
		return err
	}

	if err := c.repo.TeamValueInsert(ctx, gensql.ChartTypeAirflow, teamValueKeyDatabasePassword, values.PostgresPassword, team.ID); err != nil {
		log.WithError(err).Infof("inserting %v team value to database", teamValueKeyDatabasePassword)
		return err
	}

	if err := c.repo.TeamValueInsert(ctx, gensql.ChartTypeAirflow, teamValueKeyFernetKey, values.FernetKey, team.ID); err != nil {
		log.WithError(err).Infof("inserting %v team value to database", teamValueKeyFernetKey)
		return err
	}

	if err := c.repo.TeamValueInsert(ctx, gensql.ChartTypeAirflow, teamValueKeyWebserverSecret, values.WebserverSecretKey, team.ID); err != nil {
		log.WithError(err).Infof("inserting %v team value to database", teamValueKeyWebserverSecret)
		return err
	}

	if err := c.repo.TeamValueInsert(ctx, gensql.ChartTypeAirflow, TeamValueKeyRestrictEgress, strconv.FormatBool(values.RestrictEgress), team.ID); err != nil {
		log.WithError(err).Infof("inserting %v team value to database", TeamValueKeyRestrictEgress)
		return err
	}

	if err := c.repo.TeamValueInsert(ctx, gensql.ChartTypeAirflow, TeamValueKeyApiAccess, strconv.FormatBool(values.ApiAccess), team.ID); err != nil {
		log.WithError(err).Infof("inserting %v team value to database", TeamValueKeyApiAccess)
		return err
	}

	// Apply values to cluster
	namespace := k8s.TeamIDToNamespace(team.ID)

	if err := c.createHttpRoute(ctx, team.Slug+".airflow.knada.io", namespace, gensql.ChartTypeAirflow); err != nil {
		log.WithError(err).Info("creating http route")
		return err
	}

	if err := c.createHealtCheckPolicy(ctx, namespace, gensql.ChartTypeAirflow); err != nil {
		log.WithError(err).Info("creating health check policy")
		return err
	}

	secretStringData := map[string]string{
		"connection": fmt.Sprintf("postgresql://%v:%v@%v:5432/%v?sslmode=disable", team.ID, values.PostgresPassword, cloudSQLProxyName, team.ID),
	}
	if err := c.createOrUpdateSecret(ctx, k8sAirflowDatabaseSecretName, namespace, secretStringData); err != nil {
		log.WithError(err).Info("creating or updating airflow db secret")
		return err
	}

	secretStringData = map[string]string{"webserver-secret-key": values.WebserverSecretKey}
	if err := c.createOrUpdateSecret(ctx, k8sAirflowWebserverSecretName, namespace, secretStringData); err != nil {
		log.WithError(err).Info("creating or updating airflow webserver secret")
		return err
	}

	secretStringData = map[string]string{"fernet-key": values.FernetKey}
	if err := c.createOrUpdateSecret(ctx, k8sAirflowFernetKeySecretName, namespace, secretStringData); err != nil {
		log.WithError(err).Info("creating or updating airflow fernet key secret")
		return err
	}

	// Apply values to GCP project
	if err := c.createAirflowDatabase(ctx, &team, values.PostgresPassword); err != nil {
		log.WithError(err).Info("creating airflow database")
		return err
	}

	if err := c.createLogBucketForAirflow(ctx, team.ID); err != nil {
		log.WithError(err).Info("creating airflow log bucket")
		return err
	}

	if err := c.grantTokenCreatorRole(ctx, team.ID); err != nil {
		log.WithError(err).Info("granting SA token creator role")
		return err
	}

	return nil
}

func (c Client) deleteAirflow(ctx context.Context, teamID string, log logger.Logger) error {
	if err := c.repo.ChartDelete(ctx, teamID, gensql.ChartTypeAirflow); err != nil {
		log.WithError(err).Info("delete chart from database")
		return err
	}

	if c.dryRun {
		return nil
	}

	namespace := k8s.TeamIDToNamespace(teamID)

	if err := c.deleteHttpRoute(ctx, namespace, gensql.ChartTypeAirflow); err != nil {
		log.WithError(err).Info("deleting http route")
		return err
	}

	if err := c.deleteHealtCheckPolicy(ctx, namespace, gensql.ChartTypeAirflow); err != nil {
		log.WithError(err).Info("deleting health check policy")
		return err
	}

	if err := c.deleteCloudSQLProxyFromKubernetes(ctx, namespace); err != nil {
		log.WithError(err).Info("delete cloud sql proxy from Kubernetes")
		return err
	}

	if err := c.deleteSecretFromKubernetes(ctx, k8sAirflowDatabaseSecretName, namespace); err != nil {
		log.WithError(err).Info("delete Airflow database secret from Kubernetes")
		return err
	}

	if err := removeSQLClientIAMBinding(ctx, c.gcpProject, teamID); err != nil {
		log.WithError(err).Info("remove SQL client IAM binding")
		return err
	}

	instanceName := createAirflowcloudSQLInstanceName(teamID)
	if err := deleteCloudSQLInstance(ctx, instanceName, c.gcpProject); err != nil {
		log.WithError(err).Info("delete Cloud SQL instance from GCP")
		return err
	}

	if err := c.deleteTokenCreatorRole(ctx, teamID); err != nil {
		log.WithError(err).Info("deleting SA token creator role")
		return err
	}

	return nil
}

// mergeAirflowValues merges the values from the database with the values from the request, generate the missing values and returns the final values.
func (c Client) mergeAirflowValues(ctx context.Context, team gensql.TeamGetRow, configurableValues AirflowConfigurableValues) (AirflowValues, error) {
	if configurableValues.DagRepo == "" { // only required value
		dagRepo, err := c.repo.TeamValueGet(ctx, "dags.gitSync.repo", team.ID)
		if err != nil {
			return AirflowValues{}, err
		}

		configurableValues.DagRepo = dagRepo.Value

		dagRepoBranch, err := c.repo.TeamValueGet(ctx, "dags.gitSync.branch", team.ID)
		if err != nil {
			return AirflowValues{}, err
		}

		configurableValues.DagRepoBranch = dagRepoBranch.Value

		restrictEgressTeamValue, err := c.repo.TeamValueGet(ctx, TeamValueKeyRestrictEgress, team.ID)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return AirflowValues{}, err
			}
		} else {
			restrictEgress, err := strconv.ParseBool(restrictEgressTeamValue.Value)
			if err != nil {
				return AirflowValues{}, err
			}

			configurableValues.RestrictEgress = restrictEgress
		}

		apiAccessTeamValue, err := c.repo.TeamValueGet(ctx, TeamValueKeyApiAccess, team.ID)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return AirflowValues{}, err
			}
		} else {
			apiAccess, err := strconv.ParseBool(apiAccessTeamValue.Value)
			if err != nil {
				return AirflowValues{}, err
			}

			configurableValues.ApiAccess = apiAccess
		}

	}

	postgresPassword, err := c.getOrGeneratePassword(ctx, team.ID, teamValueKeyDatabasePassword, generatePassword)
	if err != nil {
		return AirflowValues{}, err
	}

	fernetKey, err := c.getOrGeneratePassword(ctx, team.ID, teamValueKeyFernetKey, generateFernetKey)
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

	workerLabels, err := c.createWorkerLabels(team.ID)
	if err != nil {
		return AirflowValues{}, err
	}

	webserverEnv, err := c.createAirflowWebServerEnvs(team.Users, configurableValues.ApiAccess)
	if err != nil {
		return AirflowValues{}, err
	}

	return AirflowValues{
		AirflowConfigurableValues: configurableValues,
		ExtraEnvs:                 extraEnvs,
		WorkerLabels:              workerLabels,
		FernetKey:                 fernetKey,
		PostgresPassword:          postgresPassword,
		WebserverEnv:              webserverEnv,
		WebserverSecretKey:        webserverSecretKey,
		WebserverServiceAccount:   team.ID,
		WorkerServiceAccount:      team.ID,
	}, nil
}

type airflowEnv struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (Client) createAirflowWebServerEnvs(users []string, apiAccess bool) (string, error) {
	envs := []airflowEnv{
		{
			Name:  "AIRFLOW_USERS",
			Value: strings.Join(users, ","),
		},
	}

	if apiAccess {
		envs = append(envs, airflowEnv{
			Name:  "AIRFLOW__API__AUTH_BACKENDS",
			Value: "airflow.api.auth.backend.basic_auth,airflow.api.auth.backend.session",
		})
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

func (c Client) createWorkerLabels(teamID string) (string, error) {
	labels := map[string]string{
		"team": teamID,
	}

	labelBytes, err := json.Marshal(labels)
	if err != nil {
		return "", err
	}

	return string(labelBytes), nil
}

func (c Client) createAirflowDatabase(ctx context.Context, team *gensql.TeamGetRow, dbPassword string) error {
	if c.dryRun {
		return nil
	}

	teamID := team.ID
	dbInstance := createAirflowcloudSQLInstanceName(teamID)

	if err := createCloudSQLInstance(ctx, team.Slug, dbInstance, c.gcpProject, c.gcpRegion); err != nil {
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

	return c.createOrUpdateCloudSQLProxy(ctx, "airflow-sql-proxy", teamID, k8s.TeamIDToNamespace(teamID), dbInstance)
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
	return "airflow-" + teamID + "-north"
}

func createBucketName(teamID string) string {
	return "airflow-logs-" + teamID + "-north"
}

func generatePassword() (string, error) {
	b := make([]byte, 40)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}

// generateFernetKey generates a URL-safe base64-encoded 32-byte key.
// Fernet guarantees that a message encrypted using it cannot be manipulated or read without the key.
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

func (c Client) registerAirflowHelmEvent(ctx context.Context, teamID string, eventType database.EventType, logger logger.Logger) error {
	helmEventData := helm.HelmEventData{
		TeamID:       teamID,
		Namespace:    k8s.TeamIDToNamespace(teamID),
		ReleaseName:  string(gensql.ChartTypeAirflow),
		ChartType:    gensql.ChartTypeAirflow,
		ChartRepo:    "apache-airflow",
		ChartName:    "airflow",
		ChartVersion: c.chartVersionAirflow,
	}

	if err := c.registerHelmEvent(ctx, eventType, teamID, helmEventData, logger); err != nil {
		return err
	}

	return nil
}

func (c Client) grantTokenCreatorRole(ctx context.Context, teamID string) error {
	if c.dryRun {
		return nil
	}

	return grantSATokenCreatorRole(ctx, teamID, c.gcpProject)
}

func (c Client) deleteTokenCreatorRole(ctx context.Context, teamID string) error {
	if c.dryRun {
		return nil
	}

	return deleteTokenCreatorRoleOnSA(ctx, teamID, c.gcpProject)
}
