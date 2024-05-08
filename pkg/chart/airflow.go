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

	"github.com/navikt/knorten/pkg/gcpapi"
	"github.com/navikt/knorten/pkg/k8s/cnpg"
	"github.com/navikt/knorten/pkg/k8s/core"

	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/database/gensql"
	"github.com/navikt/knorten/pkg/helm"
	"github.com/navikt/knorten/pkg/k8s"
	"github.com/navikt/knorten/pkg/reflect"
)

const (
	k8sAirflowFernetKeySecretName = "airflow-fernet-key"
	k8sAirflowWebserverSecretName = "airflow-webserver"
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
	*AirflowConfigurableValues

	// Manually save to database
	FernetKey          string // Knorten sets Helm value pointing to k8s secret
	WebserverSecretKey string // Knorten sets Helm value pointing to k8s secret

	// Generated Helm config
	ExtraEnvs               string `helm:"env"`
	WebserverEnv            string `helm:"webserver.env"`
	WebserverServiceAccount string `helm:"webserver.serviceAccount.name"`
	WorkerServiceAccount    string `helm:"workers.serviceAccount.name"`
	WorkerLabels            string `helm:"workers.labels"`
}

func (c Client) syncAirflow(ctx context.Context, configurableValues *AirflowConfigurableValues) error {
	team, err := c.repo.TeamGet(ctx, configurableValues.TeamID)
	if err != nil {
		return fmt.Errorf("getting team: %w", err)
	}

	values, err := c.mergeAirflowValues(ctx, team, configurableValues)
	if err != nil {
		return fmt.Errorf("merging airflow values: %w", err)
	}

	helmChartValues, err := reflect.CreateChartValues(values)
	if err != nil {
		return fmt.Errorf("creating helm chart values: %w", err)
	}

	// First we save all variables to the database, then we apply them to the cluster.
	if err := c.repo.HelmChartValuesInsert(ctx, gensql.ChartTypeAirflow, helmChartValues, team.ID); err != nil {
		return fmt.Errorf("inserting helm chart values to database: %w", err)
	}

	if err := c.repo.TeamValueInsert(ctx, gensql.ChartTypeAirflow, teamValueKeyFernetKey, values.FernetKey, team.ID); err != nil {
		return fmt.Errorf("inserting %v team value to database", teamValueKeyFernetKey)
	}

	if err := c.repo.TeamValueInsert(ctx, gensql.ChartTypeAirflow, teamValueKeyWebserverSecret, values.WebserverSecretKey, team.ID); err != nil {
		return fmt.Errorf("inserting %v team value to database", teamValueKeyWebserverSecret)
	}

	if err := c.repo.TeamValueInsert(ctx, gensql.ChartTypeAirflow, TeamValueKeyRestrictEgress, strconv.FormatBool(values.RestrictEgress), team.ID); err != nil {
		return fmt.Errorf("inserting %v team value to database", TeamValueKeyRestrictEgress)
	}

	if err := c.repo.TeamValueInsert(ctx, gensql.ChartTypeAirflow, TeamValueKeyApiAccess, strconv.FormatBool(values.ApiAccess), team.ID); err != nil {
		return fmt.Errorf("inserting %v team value to database", TeamValueKeyApiAccess)
	}

	// Apply values to cluster
	namespace := k8s.TeamIDToNamespace(team.ID)

	if err := c.createHttpRoute(ctx, team.Slug+".airflow."+c.topLevelDomain, namespace, gensql.ChartTypeAirflow); err != nil {
		return fmt.Errorf("creating http route: %w", err)
	}

	if err := c.createHealthCheckPolicy(ctx, namespace, gensql.ChartTypeAirflow); err != nil {
		return fmt.Errorf("creating health check policy: %w", err)
	}

	if err := c.createOrUpdateSecret(ctx, k8sAirflowWebserverSecretName, namespace, map[string]string{
		"webserver-secret-key": values.WebserverSecretKey,
	}); err != nil {
		return fmt.Errorf("creating or updating airflow webserver secret: %w", err)
	}

	if err := c.createOrUpdateSecret(ctx, k8sAirflowFernetKeySecretName, namespace, map[string]string{
		"fernet-key": values.FernetKey,
	}); err != nil {
		return fmt.Errorf("creating or updating airflow fernet key secret: %w", err)
	}

	// Apply values to GCP project
	if err := c.createAirflowDatabase(ctx, &team); err != nil {
		return fmt.Errorf("creating airflow database: %w", err)
	}

	if err := c.createLogBucketForAirflow(ctx, team.ID); err != nil {
		return fmt.Errorf("creating log bucket for airflow: %w", err)
	}

	if err := c.grantTokenCreatorRole(ctx, team.ID); err != nil {
		return fmt.Errorf("granting SA token creator role: %w", err)
	}

	return nil
}

func (c Client) deleteAirflow(ctx context.Context, teamID string) error {
	if err := c.repo.ChartDelete(ctx, teamID, gensql.ChartTypeAirflow); err != nil {
		return fmt.Errorf("deleting chart: %w", err)
	}

	if c.dryRun {
		return nil
	}

	namespace := k8s.TeamIDToNamespace(teamID)

	if err := c.deleteSecretFromKubernetes(ctx, k8sAirflowFernetKeySecretName, namespace); err != nil {
		return fmt.Errorf("deleting fernet key secret: %w", err)
	}

	if err := c.deleteSecretFromKubernetes(ctx, k8sAirflowWebserverSecretName, namespace); err != nil {
		return fmt.Errorf("deleting webserver secret: %w", err)
	}

	if err := c.deleteHttpRoute(ctx, namespace, gensql.ChartTypeAirflow); err != nil {
		return fmt.Errorf("deleting http route: %w", err)
	}

	if err := c.deleteHealthCheckPolicy(ctx, namespace, gensql.ChartTypeAirflow); err != nil {
		return fmt.Errorf("deleting health check policy: %w", err)
	}

	if err := c.manager.DeleteScheduledBackup(ctx, teamID, namespace); err != nil {
		return fmt.Errorf("deleting scheduled backup: %w", err)
	}

	if err := c.deleteCloudNativePGCluster(ctx, teamID, namespace); err != nil {
		return fmt.Errorf("deleting cloud native pg cluster: %w", err)
	}

	if err := c.deleteTokenCreatorRole(ctx, teamID); err != nil {
		return fmt.Errorf("deleting SA token creator role: %w", err)
	}

	return nil
}

// mergeAirflowValues merges the values from the database with the values from the request, generate the missing values and returns the final values.
func (c Client) mergeAirflowValues(ctx context.Context, team gensql.TeamGetRow, configurableValues *AirflowConfigurableValues) (AirflowValues, error) {
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

func teamIDToDb(name string) string {
	if strings.HasPrefix(name, "team-") {
		return strings.Replace(name, "team-", "", 1)
	} else if strings.HasPrefix(name, "team") {
		return strings.Replace(name, "team", "", 1)
	} else {
		return name
	}
}

func (c Client) createAirflowDatabase(ctx context.Context, team *gensql.TeamGetRow) error {
	if c.dryRun {
		return nil
	}

	teamID := team.ID
	dbInstance := getAirflowDatabaseName(teamID)
	namespace := k8s.TeamIDToNamespace(teamID)

	cluster := cnpg.NewCluster(
		teamIDToDb(teamID),
		namespace,
		dbInstance,
		teamID,
		cnpg.WithAppLabel("airflow-postgres"),
	)

	err := c.manager.ApplyPostgresCluster(ctx, cluster)
	if err != nil {
		return err
	}

	err = c.manager.ApplyScheduledBackup(ctx, cnpg.NewScheduledBackup(teamID, namespace, cluster.Name))
	if err != nil {
		return err
	}

	// FIXME: Should we introduce a maintenance loop that updates the airflow-db secret
	dbSecret, err := c.manager.WaitForSecret(ctx, fmt.Sprintf("%s-app", teamIDToDb(teamID)), namespace)
	if err != nil {
		return err
	}

	connectionURI, hasKey := dbSecret.Data["uri"]
	if !hasKey {
		return fmt.Errorf("missing uri key in secret %s", dbSecret.Name)
	}

	airflowDbSecret := core.NewSecret("airflow-db", namespace, map[string]string{
		"connection": string(connectionURI),
	})

	err = c.manager.ApplySecret(ctx, airflowDbSecret)
	if err != nil {
		return err
	}

	return nil
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

// FIXME: use this or remove it
// CNPG creates a secret with all the necessary information to connect to the db:
// - https://cloudnative-pg.io/documentation/1.22/applications/#secrets
// func getAirflowDatabaseSecretName(teamID string) string {
// 	return fmt.Sprintf("%s-app", getAirflowDatabaseName(teamID))
// }

func getAirflowDatabaseName(teamID string) string {
	return fmt.Sprintf("airflow-%s", teamID)
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

func (c Client) registerAirflowHelmEvent(ctx context.Context, teamID string, eventType database.EventType) error {
	helmEventData := helm.EventData{
		TeamID:       teamID,
		Namespace:    k8s.TeamIDToNamespace(teamID),
		ReleaseName:  string(gensql.ChartTypeAirflow),
		ChartType:    gensql.ChartTypeAirflow,
		ChartRepo:    "apache-airflow",
		ChartName:    "airflow",
		ChartVersion: c.chartVersionAirflow,
	}

	if err := c.registerHelmEvent(ctx, eventType, teamID, helmEventData); err != nil {
		return err
	}

	return nil
}

func (c Client) grantTokenCreatorRole(ctx context.Context, teamID string) error {
	_, err := c.saBinder.AddPolicyRole(ctx, teamID, gcpapi.ServiceAccountTokenCreatorRole)
	if err != nil {
		return err
	}

	return nil
}

func (c Client) deleteTokenCreatorRole(ctx context.Context, teamID string) error {
	exists, err := c.saChecker.Exists(ctx, teamID)
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	_, err = c.saBinder.RemovePolicyRole(ctx, teamID, gcpapi.ServiceAccountTokenCreatorRole)
	if err != nil {
		return err
	}

	return nil
}
