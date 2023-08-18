package team

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
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
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
		team gensql.Team
	}
	type want struct {
		team gensql.TeamBySlugGetRow
		err  error
	}

	operation := func(ctx context.Context, eventType database.EventType, team gensql.Team, teamClient *Client) bool {
		switch eventType {
		case database.EventTypeCreateTeam:
			return teamClient.Create(ctx, team, logrus.NewEntry(logrus.StandardLogger()))
		case database.EventTypeUpdateTeam:
			return teamClient.Update(ctx, team, logrus.NewEntry(logrus.StandardLogger()))
		case database.EventTypeDeleteTeam:
			return teamClient.Delete(ctx, team.ID, logrus.NewEntry(logrus.StandardLogger()))
		}

		return true
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
				team: gensql.Team{
					ID:    "test-team-1234",
					Slug:  "test-team",
					Users: []string{"user.one@nav.on", "user.two@nav.on"},
					Owner: "dummy@nav.no",
				},
			},
			want: want{
				team: gensql.TeamBySlugGetRow{
					ID:    "test-team-1234",
					Slug:  "test-team",
					Users: []string{"dummy@nav.no", "user.one@nav.on", "user.two@nav.on"},
					Owner: "dummy@nav.no",
				},
				err: nil,
			},
		},
		{
			name:      "Create team slug already exists",
			eventType: database.EventTypeCreateTeam,
			args: args{
				team: gensql.Team{
					ID:    "already-exists-1234",
					Slug:  "test-team",
					Users: []string{},
					Owner: "dummy@nav.no",
				},
			},
			want: want{
				team: gensql.TeamBySlugGetRow{
					ID:    "test-team-1234",
					Slug:  "test-team",
					Users: []string{"dummy@nav.no", "user.one@nav.on", "user.two@nav.on"},
					Owner: "dummy@nav.no",
				},
				err: nil,
			},
		},
		{
			name:      "Create team no users",
			eventType: database.EventTypeCreateTeam,
			args: args{
				team: gensql.Team{
					ID:    "other-team-1234",
					Slug:  "other-team",
					Users: []string{},
					Owner: "dummy@nav.no",
				},
			},
			want: want{
				team: gensql.TeamBySlugGetRow{
					ID:    "other-team-1234",
					Slug:  "other-team",
					Users: []string{"dummy@nav.no"},
					Owner: "dummy@nav.no",
				},
				err: nil,
			},
		},
		{
			name:      "Update team",
			eventType: database.EventTypeUpdateTeam,
			args: args{
				team: gensql.Team{
					ID:    "test-team-1234",
					Slug:  "test-team",
					Users: []string{"new.user@nav.no"},
				},
			},
			want: want{
				team: gensql.TeamBySlugGetRow{
					ID:    "test-team-1234",
					Slug:  "test-team",
					Users: []string{"dummy@nav.no", "new.user@nav.no"},
					Owner: "dummy@nav.no",
				},
				err: nil,
			},
		},
		{
			name:      "Delete team",
			eventType: database.EventTypeDeleteTeam,
			args: args{
				team: gensql.Team{
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
			teamClient, err := NewClient(repo, "", "", true, false)
			if err != nil {
				t.Error(err)
			}

			if retry := operation(context.Background(), tt.eventType, tt.args.team, teamClient); retry {
				t.Errorf("%v failed, got retry return for team %v", tt.eventType, tt.args.team.ID)
			}

			team, err := repo.TeamBySlugGet(context.Background(), tt.args.team.Slug)
			if err != tt.want.err {
				t.Error(err)
			}

			if diff := cmp.Diff(team, tt.want.team); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
