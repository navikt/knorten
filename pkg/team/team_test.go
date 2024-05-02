package team

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"runtime"
	"testing"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	"github.com/navikt/knorten/pkg/k8s"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/google/go-cmp/cmp"
	"github.com/navikt/knorten/local/dbsetup"
	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/database/gensql"
	"github.com/sirupsen/logrus"
)

var repo *database.Repo

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

	code := m.Run()
	os.Exit(code)
}

func TestTeam(t *testing.T) {
	ctx := context.Background()
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
		team *gensql.Team
	}
	type want struct {
		team gensql.TeamBySlugGetRow
		err  error
	}

	operation := func(ctx context.Context, eventType database.EventType, team *gensql.Team, teamClient *Client) error {
		switch eventType {
		case database.EventTypeCreateTeam:
			err := teamClient.Create(ctx, team)
			if errors.Is(err, ErrTeamExists) {
				return nil
			}

			return err
		case database.EventTypeUpdateTeam:
			return teamClient.Update(ctx, team)
		case database.EventTypeDeleteTeam:
			return teamClient.Delete(ctx, team.ID)
		}

		return fmt.Errorf("unknown event type %v", eventType)
	}

	teamTests := []struct {
		name      string
		eventType database.EventType
		args      args
		want      want
	}{
		{
			name:      "Create team",
			eventType: database.EventTypeCreateTeam,
			args: args{
				team: &gensql.Team{
					ID:    "test-team-1234",
					Slug:  "test-team",
					Users: []string{"dummy@nav.no", "user.one@nav.on", "user.two@nav.on"},
				},
			},
			want: want{
				team: gensql.TeamBySlugGetRow{
					ID:    "test-team-1234",
					Slug:  "test-team",
					Users: []string{"dummy@nav.no", "user.one@nav.on", "user.two@nav.on"},
				},
				err: nil,
			},
		},
		{
			name:      "Create team slug already exists",
			eventType: database.EventTypeCreateTeam,
			args: args{
				team: &gensql.Team{
					ID:    "already-exists-1234",
					Slug:  "test-team",
					Users: []string{"dummy@nav.no"},
				},
			},
			want: want{
				team: gensql.TeamBySlugGetRow{
					ID:    "test-team-1234",
					Slug:  "test-team",
					Users: []string{"dummy@nav.no", "user.one@nav.on", "user.two@nav.on"},
				},
				err: nil,
			},
		},
		{
			name:      "Update team",
			eventType: database.EventTypeUpdateTeam,
			args: args{
				team: &gensql.Team{
					ID:    "test-team-1234",
					Slug:  "test-team",
					Users: []string{"dummy@nav.no", "new.user@nav.no"},
				},
			},
			want: want{
				team: gensql.TeamBySlugGetRow{
					ID:    "test-team-1234",
					Slug:  "test-team",
					Users: []string{"dummy@nav.no", "new.user@nav.no"},
				},
				err: nil,
			},
		},
		{
			name:      "Delete team",
			eventType: database.EventTypeDeleteTeam,
			args: args{
				team: &gensql.Team{
					ID: "test-team-1234",
				},
			},
			want: want{
				team: gensql.TeamBySlugGetRow{},
				err:  sql.ErrNoRows,
			},
		},
	}

	for _, tt := range teamTests {
		t.Run(tt.name, func(t *testing.T) {
			// FIXME: Can add some logging of requests to this fake client thingy
			c := fake.NewFakeClient()
			scheme := c.Scheme()

			// Probably don't need these here as we are using core/v1 schemas
			if err := cnpgv1.AddToScheme(scheme); err != nil {
				t.Error(err)
			}

			if err := gwapiv1.AddToScheme(scheme); err != nil {
				t.Error(err)
			}

			teamClient, err := NewClient(repo, k8s.NewManager(&k8s.Client{
				Client:     c,
				RESTConfig: nil,
			}), "", "", true)
			if err != nil {
				t.Error(err)
			}

			err = operation(context.Background(), tt.eventType, tt.args.team, teamClient)
			if err != nil {
				t.Errorf("%v failed, got err return for team %v", tt.eventType, tt.args.team.ID)
			}

			team, err := repo.TeamBySlugGet(context.Background(), tt.args.team.Slug)
			if !errors.Is(err, tt.want.err) {
				t.Error(err)
			}

			if diff := cmp.Diff(team, tt.want.team); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
