package chart

import (
	"context"
	"log"
	"os"
	"path"
	"runtime"
	"testing"

	"github.com/nais/knorten/local/dbsetup"
	"github.com/nais/knorten/pkg/api/auth"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
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
	var err error
	repo, err = dbsetup.SetupDBForTests()
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
		if err := repo.TeamDelete(ctx, team.ID); err != nil {
			t.Error(err)
		}
	})

	type args struct {
		values JupyterConfigurableValues
	}
	type want struct {
		values    []gensql.ChartTeamValue
		numValues int
		err       error
	}

	operation := func(ctx context.Context, eventType gensql.EventType, values any, chartClient *Client) bool {
		switch eventType {
		case gensql.EventTypeCreateJupyter,
			gensql.EventTypeUpdateJupyter:
			return chartClient.SyncJupyter(ctx, values.(JupyterConfigurableValues), logrus.NewEntry(logrus.StandardLogger()))
		}

		return true
	}

	teamTests := []struct {
		name      string
		eventType gensql.EventType
		chartType gensql.ChartType
		args      args
		want      want
	}{
		{
			name:      "Create jupyter chart",
			eventType: gensql.EventTypeCreateJupyter,
			chartType: gensql.ChartTypeJupyterhub,
			args: args{
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
	}

	for _, tt := range teamTests {
		t.Run(tt.name, func(t *testing.T) {
			chartClient, err := NewClient(repo, azureClient, true, false, "1.10.0", "2.0.0", "project", "")
			if err != nil {
				t.Error(err)
			}

			if retry := operation(context.Background(), tt.eventType, tt.args.values, chartClient); retry {
				t.Error("retry")
			}

			teamValues, err := repo.TeamValuesGet(ctx, tt.chartType, team.ID)
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

				if dbVal.Value != v.Value {
					t.Errorf("chart value, expected %v, got %v", v.Value, dbVal.Value)
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
	return team, repo.TeamCreate(ctx, team)
}
