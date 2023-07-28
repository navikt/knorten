package e2etests

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"runtime"
	"testing"
	"text/template"
	"time"

	"github.com/nais/knorten/local/dbsetup"
	"github.com/nais/knorten/pkg/api"
	"github.com/nais/knorten/pkg/api/auth"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/gensql"
	"github.com/nais/knorten/pkg/events"
	"github.com/ory/dockertest/v3"
	"github.com/sirupsen/logrus"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/html"
)

var (
	repo   *database.Repo
	server *httptest.Server
	user   = auth.User{
		Name:  "Dum My",
		Email: "dummy@nav.no",
	}
)

const (
	htmlContentType = "text/html; charset=utf-8"
	jsonContentType = "application/json; charset=utf-8"
)

func init() {
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Join(path.Dir(filename), "..")
	err := os.Chdir(dir)
	if err != nil {
		panic(err)
	}
}

func TestMain(m *testing.M) {
	repo, err := setupDatabase()
	if err != nil {
		log.Fatalf("setting up database: %v", err)
	}

	eventHandler, err := events.NewHandler(context.Background(), repo, "", "", "", "", "", true, false, logrus.NewEntry(logrus.StandardLogger()))
	if err != nil {
		log.Fatalf("creating eventhandler: %v", err)
	}
	eventHandler.Run(1 * time.Second)

	srv, err := api.New(repo, true, "", "", " ", "", "nada@nav.no", "", "", logrus.NewEntry(logrus.StandardLogger()))
	if err != nil {
		log.Fatalf("creating api: %v", err)
	}

	server = httptest.NewServer(srv)

	os.Exit(m.Run())
}

func setupDatabase() (*database.Repo, error) {
	dbPort := "5432"
	dbHost := "db"
	dbName := "knorten"
	dbString := fmt.Sprintf("user=postgres dbname=%v sslmode=disable password=postgres host=%v port=%v", dbName, dbHost, dbPort)

	if os.Getenv("CI") != "true" {
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
			return nil, fmt.Errorf("could not connect to docker: %s", err)
		}

		// pulls an image, creates a container based on it and runs it
		resource, err := pool.Run("postgres", "14", []string{"POSTGRES_PASSWORD=postgres", "POSTGRES_DB=knorten"})
		if err != nil {
			return nil, fmt.Errorf("could not start resource: %s", err)
		}

		// setting resource timeout as postgres container is not terminated automatically
		if err := resource.Expire(120); err != nil {
			return nil, fmt.Errorf("failed creating postgres expire: %v", err)
		}

		dbPort = resource.GetPort("5432/tcp")
		dbHost = "localhost"
		dbString = fmt.Sprintf("user=postgres dbname=%v sslmode=disable password=postgres host=localhost port=%v", dbName, dbPort)
	}

	if err := waitForDB(dbString); err != nil {
		return nil, err
	}

	repo, err := database.New(dbString, "jegersekstentegn", logrus.NewEntry(logrus.StandardLogger()))
	if err != nil {
		return nil, err
	}

	if err := dbsetup.SetupDB(context.Background(), fmt.Sprintf("postgres://postgres:postgres@%v:%v", dbHost, dbPort), dbName); err != nil {
		return nil, fmt.Errorf("setting up knorten db: %v", err)
	}

	return repo, nil
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

func minimizeHTML(in string) (string, error) {
	m := minify.New()
	m.AddFunc("text/html", html.Minify)

	out, err := m.String("text/html", in)
	if err != nil {
		return "", err
	}

	return out, nil
}

func createTeamAndApps(repo *database.Repo, server *httptest.Server, teamSlug string) error {
	data := url.Values{"team": {teamSlug}, "owner": {user.Email}, "users[]": {"user.userson@nav.no"}, "apiaccess": {""}}
	resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/new", server.URL), data)
	if err != nil {
		return fmt.Errorf("creating team: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("creating team returned status code %v", resp.StatusCode)
	}

	team, err := waitForTeamInDatabase(repo, teamSlug)
	if err != nil {
		return err
	}

	data = url.Values{"cpu": {"1.0"}, "memory": {"1G"}, "imagename": {""}, "imagetag": {""}, "culltimeout": {"3600"}}
	resp, err = server.Client().PostForm(fmt.Sprintf("%v/team/%v/jupyterhub/new", server.URL, teamSlug), data)
	if err != nil {
		return fmt.Errorf("creating jupyterhub for team %v: %v", teamSlug, err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("creating jupyterhub for team %v returned status code: %v", teamSlug, resp.StatusCode)
	}

	if err := waitForChartInDatabase(repo, gensql.ChartTypeJupyterhub, team.ID); err != nil {
		return err
	}

	data = url.Values{"dagrepo": {"navikt/repo"}, "dagrepobranch": {"main"}, "apiaccess": {""}, "restrictairflowegress": {""}}
	resp, err = server.Client().PostForm(fmt.Sprintf("%v/team/%v/airflow/new", server.URL, teamSlug), data)
	if err != nil {
		return fmt.Errorf("creating airflow for team %v: %v", teamSlug, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("creating airflow for team %v returned status code: %v", teamSlug, resp.StatusCode)
	}

	if err := waitForChartInDatabase(repo, gensql.ChartTypeAirflow, team.ID); err != nil {
		return err
	}

	resp, err = server.Client().Post(fmt.Sprintf("%v/compute/new", server.URL), jsonContentType, nil)
	if err != nil {
		return fmt.Errorf("creating compute instance for user %v: %v", user.Email, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("creating compute instance for user %v returned status code %v", user, resp.StatusCode)
	}

	_, err = waitForComputeInstanceInDatabase(repo, user.Email)
	if err != nil {
		return err
	}

	return nil
}

func waitForComputeInstanceInDatabase(repo *database.Repo, email string) (gensql.ComputeInstance, error) {
	timeout := 60
	for timeout > 0 {
		instance, err := repo.ComputeInstanceGet(context.Background(), email)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				time.Sleep(1 * time.Second)
				timeout--
				continue
			}

			return gensql.ComputeInstance{}, err
		}

		return instance, nil
	}

	return gensql.ComputeInstance{}, fmt.Errorf("timed out waiting for compute instance for user %v to be created", email)
}

func waitForTeamInDatabase(repo *database.Repo, teamSlug string) (gensql.TeamBySlugGetRow, error) {
	timeout := 60
	for timeout > 0 {
		team, err := repo.TeamBySlugGet(context.Background(), teamSlug)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				time.Sleep(1 * time.Second)
				timeout--
				continue
			}

			return gensql.TeamBySlugGetRow{}, err
		}

		return team, nil
	}

	return gensql.TeamBySlugGetRow{}, fmt.Errorf("timed out waiting for team %v to be created", teamSlug)
}

func waitForChartInDatabase(repo *database.Repo, chartType gensql.ChartType, teamID string) error {
	timeout := 60
	for timeout > 0 {
		apps, err := repo.AppsForTeamGet(context.Background(), teamID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		for _, app := range apps {
			if app == chartType {
				return nil
			}
		}

		time.Sleep(1 * time.Second)
		timeout--
	}

	return fmt.Errorf("timed out waiting for chart %v to be created", chartType)
}

func waitForTeamToBeDeletedFromDatabase(repo *database.Repo, teamSlug string) error {
	timeout := 60
	for timeout > 0 {
		_, err := repo.TeamBySlugGet(context.Background(), teamSlug)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}

			return err
		}

		time.Sleep(1 * time.Second)
		timeout--
	}

	return fmt.Errorf("timed out waiting for team %v to be deleted", teamSlug)
}

func cleanupTeamAndApps(repo *database.Repo, server *httptest.Server, teamSlug string) error {
	resp, err := server.Client().Post(fmt.Sprintf("%v/team/%v/delete", server.URL, teamSlug), jsonContentType, nil)
	if err != nil {
		return fmt.Errorf("deleting team %v: %v", teamSlug, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("deleting team returned status code %v", resp.StatusCode)
	}

	return waitForTeamToBeDeletedFromDatabase(repo, teamSlug)
}

func createExpectedHTML(t string, values map[string]any) (string, error) {
	buff := &bytes.Buffer{}
	tmpl, err := template.ParseGlob("templates/**/*")
	if err != nil {
		return "", err
	}
	if err := tmpl.ExecuteTemplate(buff, t, values); err != nil {
		return "", err
	}

	dataBytes, err := io.ReadAll(buff)
	if err != nil {
		panic(err)
	}

	return string(dataBytes), nil
}
