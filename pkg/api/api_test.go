package api

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http/httptest"
	"os"
	"path"
	"runtime"
	"testing"
	"time"

	"github.com/nais/knorten/local/dbsetup"
	"github.com/nais/knorten/pkg/api/auth"
	"github.com/nais/knorten/pkg/database"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/html"

	"github.com/ory/dockertest/v3"
	"github.com/sirupsen/logrus"
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
	dir := path.Join(path.Dir(filename), "../..")
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

	logger := logrus.NewEntry(logrus.StandardLogger())

	var err error
	repo, err = database.New(dbString, "jegersekstentegn", logger)
	if err != nil {
		log.Fatal(err)
	}

	if err := dbsetup.SetupDB(context.Background(), fmt.Sprintf("postgres://postgres:postgres@%v:%v", dbHost, dbPort), "knorten"); err != nil {
		log.Fatalf("setting up knorten db: %v", err)
	}

	azureClient, err := auth.NewAzureClient(true, "", "", "", logger)
	if err != nil {
		log.Fatalf("creating azure client: %v", err)
	}

	srv, err := New(repo, azureClient, true, "jegersekstentegn", "nada@nav.no", "", "", logger)
	if err != nil {
		log.Fatalf("setting up api: %v", err)
	}

	server = httptest.NewServer(srv)

	code := m.Run()

	server.Close()

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
