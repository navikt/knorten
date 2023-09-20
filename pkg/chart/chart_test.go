package chart

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/knorten/local/dbsetup"
	"github.com/nais/knorten/pkg/api/auth"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/helm"
	"github.com/sirupsen/logrus"
)

var (
	repo        *database.Repo
	azureClient *auth.Azure
)

func init() {
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Join(path.Dir(filename), "../..")
	err := os.Chdir(dir)
	if err != nil {
		panic(err)
	}
}

func TestMain(m *testing.M) {
	dbConn, err := dbsetup.SetupDBForTests()
	if err != nil {
		log.Fatal(err)
	}
	repo, err = database.New(dbConn, "", logrus.NewEntry(logrus.StandardLogger()))
	if err != nil {
		log.Fatal(err)
	}

	azureClient, err = auth.NewAzureClient(true, "", "", "", logrus.NewEntry(logrus.StandardLogger()))
	if err != nil {
		log.Fatalf("creating azure client: %v", err)
	}

	code := m.Run()
	os.Exit(code)
}

func TestCharts(t *testing.T) {
	ctx := context.Background()
	team, err := prepareChartTests(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		teams, err := repo.TeamsGet(ctx)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			t.Error(err)
		}
		for _, team := range teams {
			if err := repo.TeamDelete(ctx, team.ID); err != nil {
				t.Error(err)
			}
		}
	})

	operation := func(ctx context.Context, eventType database.EventType, values any, chartClient *Client) bool {
		switch eventType {
		case database.EventTypeCreateJupyter,
			database.EventTypeUpdateJupyter:
			return chartClient.SyncJupyter(ctx, values.(JupyterConfigurableValues), logrus.NewEntry(logrus.StandardLogger()))
		case database.EventTypeDeleteJupyter:
			return chartClient.DeleteJupyter(ctx, values.(JupyterConfigurableValues).TeamID, logrus.NewEntry(logrus.StandardLogger()))
		case database.EventTypeCreateAirflow,
			database.EventTypeUpdateAirflow:
			return chartClient.SyncAirflow(ctx, values.(AirflowConfigurableValues), logrus.NewEntry(logrus.StandardLogger()))
		case database.EventTypeDeleteAirflow:
			return chartClient.DeleteAirflow(ctx, values.(AirflowConfigurableValues).TeamID, logrus.NewEntry(logrus.StandardLogger()))
		}

		return true
	}

	type args struct {
		eventType database.EventType
		chartType gensql.ChartType
		values    any
	}

	teamTests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "Create jupyter chart",
			args: args{
				eventType: database.EventTypeCreateJupyter,
				chartType: gensql.ChartTypeJupyterhub,
				values: JupyterConfigurableValues{
					TeamID:      team.ID,
					UserIdents:  []string{"d123456", "u654321"},
					CPU:         "1.0",
					Memory:      "2G",
					ImageName:   "ghcr.io/navikt/image",
					ImageTag:    "v1",
					CullTimeout: "7200",
				},
			},
			want: map[string]string{
				"cull.timeout":                                        "7200",
				"singleuser.image.name":                               "ghcr.io/navikt/image",
				"singleuser.image.tag":                                "v1",
				"singleuser.cpu.limit":                                "1.0",
				"singleuser.cpu.guarantee":                            "1.0",
				"singleuser.memory.limit":                             "2G",
				"singleuser.memory.guarantee":                         "2G",
				"hub.config.Authenticator.admin_users":                `["d123456", "u654321"]`,
				"hub.config.Authenticator.allowed_users":              `["d123456", "u654321"]`,
				"ingress.hosts":                                       `["test-team.jupyter.knada.io"]`,
				"ingress.tls":                                         `[{"hosts":["test-team.jupyter.knada.io"], "secretName": "jupyterhub-certificate"}]`,
				"hub.config.AzureAdOAuthenticator.oauth_callback_url": "https://test-team.jupyter.knada.io/hub/oauth_callback",
				"singleuser.extraEnv.KNADA_TEAM_SECRET":               `projects/project/secrets/test-team-1234`,
				"singleuser.profileList":                              `[{"display_name":"Custom image","description":"Custom image for team test-team-1234","kubespawner_override":{"image":"ghcr.io/navikt/image:v1"}}]`,
			},
		},
		{
			name: "Update jupyter chart",
			args: args{
				eventType: database.EventTypeCreateJupyter,
				chartType: gensql.ChartTypeJupyterhub,
				values: JupyterConfigurableValues{
					TeamID:      team.ID,
					UserIdents:  []string{"d123456"},
					CPU:         "1.0",
					Memory:      "4G",
					ImageName:   "ghcr.io/navikt/image",
					ImageTag:    "v2",
					CullTimeout: "7200",
				},
			},
			want: map[string]string{
				"cull.timeout":                                        "7200",
				"singleuser.image.name":                               "ghcr.io/navikt/image",
				"singleuser.image.tag":                                "v2",
				"singleuser.cpu.limit":                                "1.0",
				"singleuser.cpu.guarantee":                            "1.0",
				"singleuser.memory.limit":                             "4G",
				"singleuser.memory.guarantee":                         "4G",
				"hub.config.Authenticator.admin_users":                `["d123456"]`,
				"hub.config.Authenticator.allowed_users":              `["d123456"]`,
				"ingress.hosts":                                       `["test-team.jupyter.knada.io"]`,
				"ingress.tls":                                         `[{"hosts":["test-team.jupyter.knada.io"], "secretName": "jupyterhub-certificate"}]`,
				"hub.config.AzureAdOAuthenticator.oauth_callback_url": "https://test-team.jupyter.knada.io/hub/oauth_callback",
				"singleuser.extraEnv.KNADA_TEAM_SECRET":               `projects/project/secrets/test-team-1234`,
				"singleuser.profileList":                              `[{"display_name":"Custom image","description":"Custom image for team test-team-1234","kubespawner_override":{"image":"ghcr.io/navikt/image:v2"}}]`,
			},
		},
		{
			name: "Delete jupyter chart",
			args: args{
				eventType: database.EventTypeDeleteJupyter,
				chartType: gensql.ChartTypeJupyterhub,
				values: JupyterConfigurableValues{
					TeamID: team.ID,
				},
			},
			want: map[string]string{},
		},
		{
			name: "Create airflow chart",
			args: args{
				eventType: database.EventTypeCreateAirflow,
				chartType: gensql.ChartTypeAirflow,
				values: AirflowConfigurableValues{
					TeamID:        team.ID,
					DagRepo:       "navikt/my-dags",
					DagRepoBranch: "main",
				},
			},
			want: map[string]string{
				"webserver.env":                 `[{"name":"AIRFLOW_USERS","value":"dummy@nav.no,user.one@nav.no"}]`,
				"dags.gitSync.repo":             "navikt/my-dags",
				"dags.gitSync.branch":           "main",
				"webserver.serviceAccount.name": "test-team-1234",
				"workers.serviceAccount.name":   "test-team-1234",
				"env":                           `[{"name":"KNADA_TEAM_SECRET","value":"projects/project/secrets/test-team-1234"},{"name":"TEAM","value":"test-team-1234"},{"name":"NAMESPACE","value":"team-test-team-1234"},{"name":"AIRFLOW__LOGGING__REMOTE_BASE_LOG_FOLDER","value":"gs://airflow-logs-test-team-1234-north"},{"name":"AIRFLOW__LOGGING__REMOTE_LOGGING","value":"True"}]`,
				"ingress.web.hosts":             `[{"name":"test-team.airflow.knada.io","tls":{"enabled":true,"secretName":"airflow-certificate"}}]`,
			},
		},
		{
			name: "Update airflow chart",
			args: args{
				eventType: database.EventTypeUpdateAirflow,
				chartType: gensql.ChartTypeAirflow,
				values: AirflowConfigurableValues{
					TeamID:        team.ID,
					DagRepo:       "navikt/other-dags",
					DagRepoBranch: "master",
				},
			},
			want: map[string]string{
				"workers.serviceAccount.name":   "test-team-1234",
				"webserver.serviceAccount.name": "test-team-1234",
				"env":                           `[{"name":"KNADA_TEAM_SECRET","value":"projects/project/secrets/test-team-1234"},{"name":"TEAM","value":"test-team-1234"},{"name":"NAMESPACE","value":"team-test-team-1234"},{"name":"AIRFLOW__LOGGING__REMOTE_BASE_LOG_FOLDER","value":"gs://airflow-logs-test-team-1234-north"},{"name":"AIRFLOW__LOGGING__REMOTE_LOGGING","value":"True"}]`,
				"webserver.env":                 `[{"name":"AIRFLOW_USERS","value":"dummy@nav.no,user.one@nav.no"}]`,
				"ingress.web.hosts":             `[{"name":"test-team.airflow.knada.io","tls":{"enabled":true,"secretName":"airflow-certificate"}}]`,
				"dags.gitSync.repo":             "navikt/other-dags",
				"dags.gitSync.branch":           "master",
			},
		},
		{
			name: "Delete airflow chart",
			args: args{
				eventType: database.EventTypeDeleteAirflow,
				chartType: gensql.ChartTypeAirflow,
				values: AirflowConfigurableValues{
					TeamID: team.ID,
				},
			},
			want: map[string]string{},
		},
	}

	for _, tt := range teamTests {
		t.Run(tt.name, func(t *testing.T) {
			chartClient, err := NewClient(repo, azureClient, true, false, "1.10.0", "2.0.0", "project", "")
			if err != nil {
				t.Error(err)
			}

			if retry := operation(ctx, tt.args.eventType, tt.args.values, chartClient); retry {
				t.Errorf("%v failed: got retry return", tt.args.eventType)
			}

			teamValues, err := repo.TeamValuesGet(ctx, tt.args.chartType, team.ID)
			if err != nil {
				t.Fatal(err)
			}

			databaseValues := map[string]string{}
			for _, teamValue := range teamValues {
				if strings.HasSuffix(teamValue.Key, ",omit") {
					continue
				}

				databaseValues[teamValue.Key] = teamValue.Value
			}

			if diff := cmp.Diff(tt.want, databaseValues); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func prepareChartTests(ctx context.Context) (gensql.Team, error) {
	team := gensql.Team{
		ID:    "test-team-1234",
		Slug:  "test-team",
		Users: []string{"dummy@nav.no", "user.one@nav.no"},
	}

	if err := helm.UpdateHelmRepositories(); err != nil {
		return gensql.Team{}, err
	}

	return team, repo.TeamCreate(ctx, team)
}
