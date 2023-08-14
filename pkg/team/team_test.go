package team

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path"
	"runtime"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/knorten/pkg/api/auth"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/ory/dockertest/v3"
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
	dbString := "user=postgres dbname=knorten sslmode=disable password=postgres host=db port=5432"

	if os.Getenv("CLOUDBUILD") != "true" {
		dockerHost := os.Getenv("HOME") + "/.colima/docker.sock"
		_, err := os.Stat(dockerHost)
		if err != nil {
			// uses a sensible default on windows (tcp/http) and linux/osx (socket)
			dockerHost = ""
		} else {
			dockerHost = "unix://" + dockerHost
		}

		pool, err := dockertest.NewPool(dockerHost)
		if err != nil {
			log.Fatalf("Could not connect to docker: %s", err)
		}

		// pulls an image, creates a container based on it and runs it
		resource, err := pool.Run("postgres", "14", []string{"POSTGRES_PASSWORD=postgres", "POSTGRES_DB=knorten"})
		if err != nil {
			log.Fatalf("Could not start resource: %s", err)
		}

		// setting resource timeout as postgres container is not terminated automatically
		if err = resource.Expire(120); err != nil {
			log.Fatalf("failed creating postgres expire: %v", err)
		}

		dbPort := resource.GetPort("5432/tcp")
		dbString = fmt.Sprintf("user=postgres dbname=knorten sslmode=disable password=postgres host=localhost port=%v", dbPort)
	}

	if err := waitForDB(dbString); err != nil {
		log.Fatal(err)
	}

	logger := logrus.NewEntry(logrus.StandardLogger())

	var err error
	repo, err = database.New(dbString, "jegersekstentegn", logger)
	if err != nil {
		log.Fatal(err)
	}

	_, err = auth.NewAzureClient(true, "", "", "", logger)
	if err != nil {
		log.Fatalf("creating azure client: %v", err)
	}

	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})

	code := m.Run()

	os.Exit(code)
}

func waitForDB(dbString string) error {
	sleepDuration := 1 * time.Second
	numRetries := 60
	for i := 0; i < numRetries; i++ {
		time.Sleep(sleepDuration)
		db, err := sql.Open("postgres", dbString)
		if err != nil {
			return err
		}

		if err := db.Ping(); err == nil {
			return nil
		}
	}

	return fmt.Errorf("unable to connect to db in %v seconds", int(sleepDuration)*numRetries/1000000000)
}

func TestTeam(t *testing.T) {
	type args struct {
		team gensql.Team
	}
	type want struct {
		team gensql.TeamGetRow
		err  error
	}

	operation := func(ctx context.Context, eventType gensql.EventType, team gensql.Team, teamClient *Client) bool {
		switch eventType {
		case gensql.EventTypeCreateTeam:
			return teamClient.Create(ctx, team, logrus.NewEntry(logrus.StandardLogger()))
		case gensql.EventTypeUpdateTeam:
			return teamClient.Update(ctx, team, logrus.NewEntry(logrus.StandardLogger()))
		case gensql.EventTypeDeleteTeam:
			return teamClient.Delete(ctx, team.ID, logrus.NewEntry(logrus.StandardLogger()))
		}

		return true
	}

	teamTests := []struct {
		name      string
		eventType gensql.EventType
		args      args
		want      want
	}{
		{
			name:      "Create team",
			eventType: gensql.EventTypeCreateTeam,
			args: args{
				team: gensql.Team{
					ID:    "test-team-1234",
					Slug:  "test-team",
					Users: []string{"user.one@nav.on", "user.two@nav.on"},
					Owner: "dummy@nav.no",
				},
			},
			want: want{
				team: gensql.TeamGetRow{
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
			eventType: gensql.EventTypeCreateTeam,
			args: args{
				team: gensql.Team{
					ID:    "other-team-1234",
					Slug:  "other-team",
					Users: []string{},
					Owner: "dummy@nav.no",
				},
			},
			want: want{
				team: gensql.TeamGetRow{
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
			eventType: gensql.EventTypeUpdateTeam,
			args: args{
				team: gensql.Team{
					ID:    "test-team-1234",
					Slug:  "test-team",
					Users: []string{"new.user@nav.no"},
				},
			},
			want: want{
				team: gensql.TeamGetRow{
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
			eventType: gensql.EventTypeDeleteTeam,
			args: args{
				team: gensql.Team{
					ID: "test-team-1234",
				},
			},
			want: want{
				team: gensql.TeamGetRow{
					ID:    "",
					Slug:  "",
					Users: nil,
					Owner: "",
				},
				err: sql.ErrNoRows,
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

			if team.ID != tt.want.team.ID {
				t.Errorf("team id, expected %v, got %v", tt.want.team.ID, team.ID)
			}

			if team.Slug != tt.want.team.Slug {
				t.Errorf("team slug, expected %v, got %v", tt.want.team.Slug, team.Slug)
			}

			if team.Owner != tt.want.team.Owner {
				t.Errorf("team owner, expected %v, got %v", tt.want.team.Owner, team.Owner)
			}

			if diff := cmp.Diff(team.Users, tt.want.team.Users); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
