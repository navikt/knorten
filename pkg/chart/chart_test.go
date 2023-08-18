package chart

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"path"
	"runtime"
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

	type args struct {
		eventType database.EventType
		chartType gensql.ChartType
		values    any
	}
	type want struct {
		values    []gensql.ChartTeamValue
		numValues int
	}

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

	teamTests := []struct {
		name string
		args args
		want want
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
					// AllowList:   []string{"data.nav.no", "pypi.org"},
				},
			},
			want: want{
				values: []gensql.ChartTeamValue{
					{
						Key:   "singleuser.cpu.limit",
						Value: "1.0",
					},
					{
						Key:   "singleuser.cpu.guarantee",
						Value: "1.0",
					},
					{
						Key:   "singleuser.memory.limit",
						Value: "2G",
					},
					{
						Key:   "singleuser.memory.limit",
						Value: "2G",
					},
					{
						Key:   "hub.config.Authenticator.admin_users",
						Value: `["d123456", "u654321"]`,
					},
					{
						Key:   "hub.config.Authenticator.allowed_users",
						Value: `["d123456", "u654321"]`,
					},
					{
						Key:   "ingress.hosts",
						Value: `["test-team.jupyter.knada.io"]`,
					},
					{
						Key:   "ingress.tls",
						Value: `[{"hosts":["test-team.jupyter.knada.io"], "secretName": "jupyterhub-certificate"}]`,
					},
					{
						Key:   "hub.config.AzureAdOAuthenticator.oauth_callback_url",
						Value: "https://test-team.jupyter.knada.io/hub/oauth_callback",
					},
					{
						Key:   "singleuser.extraEnv.KNADA_TEAM_SECRET",
						Value: `projects/project/secrets/test-team-1234`,
					},
					{
						Key:   "singleuser.profileList",
						Value: `[{"display_name":"Custom image","description":"Custom image for team test-team-1234","kubespawner_override":{"image":"ghcr.io/navikt/image:v1"}}]`,
					},
				},
				numValues: 14,
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
					// AllowList:   []string{"data.nav.no", "pypi.org"},
				},
			},
			want: want{
				values: []gensql.ChartTeamValue{
					{
						Key:   "singleuser.cpu.limit",
						Value: "1.0",
					},
					{
						Key:   "singleuser.cpu.guarantee",
						Value: "1.0",
					},
					{
						Key:   "singleuser.memory.limit",
						Value: "4G",
					},
					{
						Key:   "singleuser.memory.limit",
						Value: "4G",
					},
					{
						Key:   "hub.config.Authenticator.admin_users",
						Value: `["d123456"]`,
					},
					{
						Key:   "hub.config.Authenticator.allowed_users",
						Value: `["d123456"]`,
					},
					{
						Key:   "ingress.hosts",
						Value: `["test-team.jupyter.knada.io"]`,
					},
					{
						Key:   "ingress.tls",
						Value: `[{"hosts":["test-team.jupyter.knada.io"], "secretName": "jupyterhub-certificate"}]`,
					},
					{
						Key:   "hub.config.AzureAdOAuthenticator.oauth_callback_url",
						Value: "https://test-team.jupyter.knada.io/hub/oauth_callback",
					},
					{
						Key:   "singleuser.extraEnv.KNADA_TEAM_SECRET",
						Value: `projects/project/secrets/test-team-1234`,
					},
					{
						Key:   "singleuser.profileList",
						Value: `[{"display_name":"Custom image","description":"Custom image for team test-team-1234","kubespawner_override":{"image":"ghcr.io/navikt/image:v2"}}]`,
					},
				},
				numValues: 14,
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
			want: want{
				values:    []gensql.ChartTeamValue{},
				numValues: 0,
			},
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
			want: want{
				values: []gensql.ChartTeamValue{
					{
						Key:   "webserver.extraContainers.[0].args.[0]",
						Value: "navikt/my-dags",
					},
					{
						Key:   "webserver.extraContainers.[0].args.[1]",
						Value: "main",
					},
					{
						Key:   "scheduler.extraInitContainers.[0].args.[0]",
						Value: "navikt/my-dags",
					},
					{
						Key:   "scheduler.extraInitContainers.[0].args.[1]",
						Value: "main",
					},
					{
						Key:   "scheduler.extraContainers.[0].args.[0]",
						Value: "navikt/my-dags",
					},
					{
						Key:   "scheduler.extraContainers.[0].args.[1]",
						Value: "main",
					},
					{
						Key:   "workers.extraInitContainers.[0].args.[0]",
						Value: "navikt/my-dags",
					},
					{
						Key:   "workers.extraInitContainers.[0].args.[1]",
						Value: "main",
					},
					{
						Key:   "webserver.serviceAccount.name",
						Value: "test-team-1234",
					},
					{
						Key:   "workers.serviceAccount.name",
						Value: "test-team-1234",
					},
					{
						Key:   "env",
						Value: `[{"name":"KNADA_TEAM_SECRET","value":"projects/project/secrets/test-team-1234"},{"name":"TEAM","value":"test-team-1234"},{"name":"NAMESPACE","value":"team-test-team-1234"},{"name":"AIRFLOW__LOGGING__REMOTE_BASE_LOG_FOLDER","value":"gs://airflow-logs-test-team-1234"},{"name":"AIRFLOW__LOGGING__REMOTE_LOGGING","value":"True"}]`,
					},
					{
						Key:   "ingress.web.hosts",
						Value: `[{"name":"test-team.airflow.knada.io","tls":{"enabled":true,"secretName":"airflow-certificate"}}]`,
					},
				},
				numValues: 17,
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
			want: want{
				values: []gensql.ChartTeamValue{
					{
						Key:   "webserver.extraContainers.[0].args.[0]",
						Value: "navikt/other-dags",
					},
					{
						Key:   "webserver.extraContainers.[0].args.[1]",
						Value: "master",
					},
					{
						Key:   "scheduler.extraInitContainers.[0].args.[0]",
						Value: "navikt/other-dags",
					},
					{
						Key:   "scheduler.extraInitContainers.[0].args.[1]",
						Value: "master",
					},
					{
						Key:   "scheduler.extraContainers.[0].args.[0]",
						Value: "navikt/other-dags",
					},
					{
						Key:   "scheduler.extraContainers.[0].args.[1]",
						Value: "master",
					},
					{
						Key:   "workers.extraInitContainers.[0].args.[0]",
						Value: "navikt/other-dags",
					},
					{
						Key:   "workers.extraInitContainers.[0].args.[1]",
						Value: "master",
					},
				},
				numValues: 17,
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
			want: want{
				values:    []gensql.ChartTeamValue{},
				numValues: 0,
			},
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

			if len(teamValues) != tt.want.numValues {
				t.Errorf("expected %v team values, got %v", tt.want.numValues, len(teamValues))
			}

			for _, v := range tt.want.values {
				dbVal, err := repo.TeamValueGet(ctx, v.Key, team.ID)
				if err != nil {
					t.Error(err)
				}

				if diff := cmp.Diff(dbVal.Value, v.Value); diff != "" {
					t.Errorf("mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func prepareChartTests(ctx context.Context) (gensql.Team, error) {
	team := gensql.Team{
		ID:    "test-team-1234",
		Slug:  "test-team",
		Users: []string{"user.one@nav.no"},
		Owner: "dummy@nav.no",
	}

	if err := helm.UpdateHelmRepositories(); err != nil {
		return gensql.Team{}, err
	}

	// global values for airflow
	globalValues := map[string]string{
		"webserver.extraContainers":     `[{"name": "git-nada", "image": "registry.k8s.io/git-sync/git-sync:v3.6.3","args": ["", "", "/dags", "60"], "volumeMounts":[{"mountPath":"/dags","name":"dags"}]}]`,
		"scheduler.extraContainers":     `[{"name": "git-nada", "image": "registry.k8s.io/git-sync/git-sync:v3.6.3","args": ["", "", "/dags", "60"], "volumeMounts":[{"mountPath":"/dags","name":"dags"}]}]`,
		"scheduler.extraInitContainers": `[{"name": "git-nada-clone", "image": "registry.k8s.io/git-sync/git-sync:v3.6.3","args": ["", "", "/dags", "60"], "volumeMounts":[{"mountPath":"/dags","name":"dags"}]}]`,
		"workers.extraInitContainers":   `[{"name": "git-nada", "image": "registry.k8s.io/git-sync/git-sync:v3.6.3","args": ["", "", "/dags", "60"], "volumeMounts":[{"mountPath":"/dags","name":"dags"}]}]`,
	}

	for k, v := range globalValues {
		if err := repo.GlobalChartValueInsert(ctx, k, v, false, gensql.ChartTypeAirflow); err != nil {
			return gensql.Team{}, err
		}
	}

	return team, repo.TeamCreate(ctx, team)
}
