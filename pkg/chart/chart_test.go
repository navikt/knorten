package chart

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	"github.com/navikt/knorten/pkg/gcpapi"
	"github.com/navikt/knorten/pkg/gcpapi/mock"
	"github.com/navikt/knorten/pkg/k8s"
	"google.golang.org/api/iam/v1"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gwapiv1b1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/google/go-cmp/cmp"
	"github.com/navikt/knorten/local/dbsetup"
	"github.com/navikt/knorten/pkg/api/auth"
	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/database/gensql"
	"github.com/navikt/knorten/pkg/helm"
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

	azureClient, err = auth.NewAzureClient(true, "", "", "", "", logrus.NewEntry(logrus.StandardLogger()))
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

	operation := func(ctx context.Context, eventType database.EventType, values any, chartClient *Client) error {
		switch eventType {
		case database.EventTypeCreateJupyter,
			database.EventTypeUpdateJupyter:
			return chartClient.SyncJupyter(ctx, values.(*JupyterConfigurableValues))
		case database.EventTypeDeleteJupyter:
			return chartClient.DeleteJupyter(ctx, values.(*JupyterConfigurableValues).TeamID)
		case database.EventTypeCreateAirflow,
			database.EventTypeUpdateAirflow:
			return chartClient.SyncAirflow(ctx, values.(*AirflowConfigurableValues))
		case database.EventTypeDeleteAirflow:
			return chartClient.DeleteAirflow(ctx, values.(*AirflowConfigurableValues).TeamID)
		}

		return nil
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
				values: &JupyterConfigurableValues{
					TeamID:        team.ID,
					UserIdents:    []string{"d123456", "u654321"},
					CPULimit:      "1.0",
					CPURequest:    "1.0",
					MemoryLimit:   "2G",
					MemoryRequest: "1G",
					ImageName:     "ghcr.io/navikt/image",
					ImageTag:      "v1",
					CullTimeout:   "7200",
				},
			},
			want: map[string]string{
				"cull.timeout":                                        "7200",
				"singleuser.image.name":                               "ghcr.io/navikt/image",
				"singleuser.image.tag":                                "v1",
				"singleuser.cpu.limit":                                "1.0",
				"singleuser.cpu.guarantee":                            "1.0",
				"singleuser.memory.limit":                             "2G",
				"singleuser.memory.guarantee":                         "1G",
				"singleuser.extraLabels":                              `{"team": "test-team-1234", "hub.jupyter.org/network-access-hub": "true"}`,
				"hub.config.Authenticator.admin_users":                `["d123456", "u654321"]`,
				"hub.config.Authenticator.allowed_users":              `["d123456", "u654321"]`,
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
				values: &JupyterConfigurableValues{
					TeamID:        team.ID,
					UserIdents:    []string{"d123456"},
					CPULimit:      "1.0",
					MemoryLimit:   "4G",
					MemoryRequest: "2G",
					ImageName:     "ghcr.io/navikt/image",
					ImageTag:      "v2",
					CullTimeout:   "7200",
				},
			},
			want: map[string]string{
				"cull.timeout":                                        "7200",
				"singleuser.image.name":                               "ghcr.io/navikt/image",
				"singleuser.image.tag":                                "v2",
				"singleuser.cpu.limit":                                "1.0",
				"singleuser.cpu.guarantee":                            "1.0",
				"singleuser.memory.limit":                             "4G",
				"singleuser.memory.guarantee":                         "2G",
				"singleuser.extraLabels":                              `{"team": "test-team-1234", "hub.jupyter.org/network-access-hub": "true"}`,
				"hub.config.Authenticator.admin_users":                `["d123456"]`,
				"hub.config.Authenticator.allowed_users":              `["d123456"]`,
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
				values: &JupyterConfigurableValues{
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
				values: &AirflowConfigurableValues{
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
				"workers.labels":                `{"team":"test-team-1234"}`,
				"env":                           `[{"name":"KNADA_TEAM_SECRET","value":"projects/project/secrets/test-team-1234"},{"name":"TEAM","value":"test-team-1234"},{"name":"NAMESPACE","value":"team-test-team-1234"},{"name":"AIRFLOW__LOGGING__REMOTE_BASE_LOG_FOLDER","value":"gs://airflow-logs-test-team-1234-north"},{"name":"AIRFLOW__LOGGING__REMOTE_LOGGING","value":"True"}]`,
			},
		},
		{
			name: "Update airflow chart",
			args: args{
				eventType: database.EventTypeUpdateAirflow,
				chartType: gensql.ChartTypeAirflow,
				values: &AirflowConfigurableValues{
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
				"workers.labels":                `{"team":"test-team-1234"}`,
				"dags.gitSync.repo":             "navikt/other-dags",
				"dags.gitSync.branch":           "master",
			},
		},
		{
			name: "Delete airflow chart",
			args: args{
				eventType: database.EventTypeDeleteAirflow,
				chartType: gensql.ChartTypeAirflow,
				values: &AirflowConfigurableValues{
					TeamID: team.ID,
				},
			},
			want: map[string]string{},
		},
	}

	for _, tt := range teamTests {
		t.Run(tt.name, func(t *testing.T) {
			// FIXME: Can add some logging of requests to this fake client thingy
			c := fake.NewFakeClient()
			scheme := c.Scheme()

			if err := cnpgv1.AddToScheme(scheme); err != nil {
				t.Error(err)
			}

			if err := gwapiv1b1.AddToScheme(scheme); err != nil {
				t.Error(err)
			}

			fetcher := mock.NewServiceAccountFetcher(&iam.ServiceAccount{}, nil)
			manager := mock.NewServiceAccountPolicyManager(&iam.Policy{}, nil)

			chartClient, err := NewClient(
				repo,
				azureClient,
				k8s.NewManager(&k8s.Client{
					Client: c,
				}),
				gcpapi.NewServiceAccountPolicyBinder("project", manager),
				gcpapi.NewServiceAccountChecker("project", fetcher),
				true,
				"1.10.0",
				"2.0.0",
				"project",
				"",
				"knada.io",
			)
			if err != nil {
				t.Error(err)
			}

			err = operation(ctx, tt.args.eventType, tt.args.values, chartClient)
			if err != nil {
				t.Errorf("got unexpected error: %v", err)
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

	h := helm.NewHelm(&helm.Config{
		RepositoryConfig: ".helm-repositories.yaml",
		Out:              io.Discard,
		Err:              io.Discard,
	})

	if err := h.Update(ctx); err != nil {
		return gensql.Team{}, err
	}

	return team, repo.TeamCreate(ctx, &team)
}
