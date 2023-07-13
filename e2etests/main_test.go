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
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/nais/knorten/local/dbsetup"
	"github.com/nais/knorten/pkg/api"
	"github.com/nais/knorten/pkg/auth"
	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/crypto"
	"github.com/nais/knorten/pkg/events"
	"github.com/nais/knorten/pkg/google"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/ory/dockertest/v3"
	"github.com/sirupsen/logrus"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/html"
)

var (
	repo   *database.Repo
	server *httptest.Server
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
	dbPort := "5432"
	dbHost := "db"
	dbString := "user=postgres dbname=knorten sslmode=disable password=postgres host=db port=5432"

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
			log.Fatalf("Could not connect to docker: %s", err)
		}

		// pulls an image, creates a container based on it and runs it
		resource, err := pool.Run("postgres", "14", []string{"POSTGRES_PASSWORD=postgres", "POSTGRES_DB=knorten"})
		if err != nil {
			log.Fatalf("Could not start resource: %s", err)
		}
		err = resource.Expire(120) // setting resource timeout as postgres container is not terminated automatically
		if err != nil {
			log.Fatalf("failed creating postgres expire: %v", err)
		}
		dbPort = resource.GetPort("5432/tcp")
		dbHost = "localhost"
		dbString = fmt.Sprintf("user=postgres dbname=knorten sslmode=disable password=postgres host=localhost port=%v", dbPort)
	}

	if err := waitForDB(dbString); err != nil {
		log.Fatal(err)
	}

	dbRepo, err := database.New(dbString, logrus.NewEntry(logrus.StandardLogger()))
	if err != nil {
		log.Fatal(err)
	}
	repo = dbRepo

	if err := dbsetup.SetupDB(context.Background(), fmt.Sprintf("postgres://postgres:postgres@%v:%v", dbHost, dbPort), "knorten"); err != nil {
		log.Fatalf("setting up knorten db: %v", err)
	}

	cryptoClient := crypto.New("jegersekstentegn")
	logger := logrus.NewEntry(logrus.StandardLogger())

	k8sClient, err := k8s.New(cryptoClient, dbRepo, true, false, "", "", "", "", "", logger)
	if err != nil {
		log.Fatalf("creating k8sClient: %v", err)
	}

	googleClient := google.New(dbRepo, "", "", true, logger)
	azureClient := auth.New(true, "", "", "", "", logger)
	chartClient, err := chart.New(dbRepo, googleClient, k8sClient, azureClient, cryptoClient, "", "", logger)
	if err != nil {
		log.Fatalf("creating googleClient: %v", err)
	}

	events.Start(context.Background(), dbRepo, "", true, false, logger)

	srv, err := api.New(dbRepo, azureClient, googleClient, k8sClient, cryptoClient, chartClient, true, "1.8.0", "2.0.0", "nada@nav.no", "session", logrus.NewEntry(logrus.StandardLogger()))
	if err != nil {
		log.Fatalf("creating api: %v", err)
	}

	server = httptest.NewServer(srv)

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

func minimizeHTML(in string) (string, error) {
	m := minify.New()
	m.AddFunc("text/html", html.Minify)

	out, err := m.String("text/html", in)
	if err != nil {
		return "", err
	}

	return out, nil
}

func replaceGeneratedValues(expected []byte, teamName string) ([]byte, error) {
	team, err := repo.TeamGet(context.Background(), teamName)
	if err != nil {
		return nil, err
	}

	fernetKey, err := repo.TeamValueGet(context.Background(), "fernetKey", team.ID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}

	updated := strings.ReplaceAll(string(expected), "${TEAM_ID}", team.ID)
	updated = strings.ReplaceAll(updated, "${FERNET_KEY}", fernetKey.Value)
	return []byte(updated), nil
}

func createTeamAndApps(teamName string) error {
	data := url.Values{"team": {teamName}, "owner": {"dummy@nav.no"}, "users[]": {"annenbruker@nav.no"}, "apiaccess": {""}}
	resp, err := server.Client().PostForm(fmt.Sprintf("%v/team/new", server.URL), data)
	if err != nil {
		return fmt.Errorf("creating team: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("creating team returned status code %v", resp.StatusCode)
	}

	data = url.Values{"cpu": {"1.0"}, "memory": {"1G"}, "imagename": {""}, "imagetag": {""}, "culltimeout": {"3600"}}
	resp, err = server.Client().PostForm(fmt.Sprintf("%v/team/%v/jupyterhub/new", server.URL, teamName), data)
	if err != nil {
		return fmt.Errorf("creating jupyterhub for team %v: %v", teamName, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("creating jupyterhub for team %v returned status code: %v", teamName, resp.StatusCode)
	}

	data = url.Values{"dagrepo": {"navikt/repo"}, "dagrepobranch": {"main"}, "apiaccess": {""}, "restrictairflowegress": {""}}
	resp, err = server.Client().PostForm(fmt.Sprintf("%v/team/%v/airflow/new", server.URL, teamName), data)
	if err != nil {
		return fmt.Errorf("creating airflow for team %v: %v", teamName, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("creating airflow for team %v returned status code: %v", teamName, resp.StatusCode)
	}

	resp, err = server.Client().Post(fmt.Sprintf("%v/compute/new", server.URL), jsonContentType, nil)
	if err != nil {
		return fmt.Errorf("creating compute instance for team %v: %v", teamName, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("creating compute instance for team %v returned status code %v", teamName, resp.StatusCode)
	}

	return nil
}

func cleanupTeamAndApps(teamName string) error {
	resp, err := server.Client().Post(fmt.Sprintf("%v/team/%v/delete", server.URL, teamName), jsonContentType, nil)
	if err != nil {
		return fmt.Errorf("deleting team %v: %v", teamName, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("deleting team returned status code %v", resp.StatusCode)
	}

	return nil
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
