package api

import (
	"bytes"
	"database/sql"
	"html/template"
	"io"
	"log"
	"net/http/httptest"
	"os"
	"path"
	"runtime"
	"testing"
	"time"

	"github.com/navikt/knorten/pkg/api/service"
	"github.com/navikt/knorten/pkg/config"
	"github.com/navikt/knorten/pkg/k8s"
	"github.com/navikt/knorten/pkg/maintenance"
	"github.com/navikt/knorten/pkg/team"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/navikt/knorten/pkg/api/handlers"

	"github.com/navikt/knorten/pkg/api/middlewares"

	"github.com/gin-gonic/gin"
	"github.com/navikt/knorten/local/dbsetup"
	"github.com/navikt/knorten/pkg/api/auth"
	"github.com/navikt/knorten/pkg/database"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/html"

	"github.com/sirupsen/logrus"
)

var (
	repo                 *database.Repo
	db                   *sql.DB
	server               *httptest.Server
	mockTeamkatalogTeams = []service.TeamkatalogTeam{
		{ID: "1", Name: "team-alpha"},
		{ID: "2", Name: "team-beta"},
		{ID: "3", Name: "team-gamma"},
	}
	testUser = auth.User{
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
	logger := logrus.NewEntry(logrus.StandardLogger())

	dbConn, err := dbsetup.SetupDBForTests()
	if err != nil {
		log.Fatal(err)
	}
	repo, err = database.New(dbConn, "", logrus.NewEntry(logrus.StandardLogger()))
	if err != nil {
		log.Fatal(err)
	}
	db, err = sql.Open("postgres", dbConn)
	if err != nil {
		log.Fatalf("open sql connection: %v", err)
	}

	azureClient, err := auth.NewAzureClient(true, "", "", "", "", logger)
	if err != nil {
		log.Fatalf("creating azure client: %v", err)
	}

	router := gin.New()

	session, err := repo.NewSessionStore("knorten_session")
	if err != nil {
		log.Fatalf("creating session store: %v", err)
	}
	router.Use(session)
	router.Static("/assets", "./assets")
	router.FuncMap = template.FuncMap{
		"toArray": toArray,
	}
	router.LoadHTMLGlob("templates/**/*")

	authService := service.NewAuthService(
		repo,
		"nada@nav.no",
		1*time.Hour,
		32,
		azureClient,
	)

	authHandler := handlers.NewAuthHandler(
		authService,
		"http://localhost:8080/",
		config.Cookies{},
		logger,
		repo,
	)

	router.Use(
		middlewares.SetSessionStatus(
			logger.WithField("subsystem", "status_middleware"),
			"knorten_session",
			repo,
		),
	)
	router.GET("/", handlers.IndexHandler)
	router.GET("/oauth2/login", authHandler.LoginHandler(true))
	router.GET("/oauth2/callback", authHandler.CallbackHandler())
	router.GET("/oauth2/logout", authHandler.LogoutHandler())
	router.Use(middlewares.Authenticate(
		logger,
		repo,
		azureClient,
		true,
	))

	c := fake.NewFakeClient()

	manager := k8s.NewManager(&k8s.Client{
		Client:     c,
		RESTConfig: nil,
	})

	err = New(
		router,
		repo,
		azureClient,
		logger,
		true,
		"",
		"",
		"test.io",
		&maintenance.MaintenanceExclusion{
			Periods: map[string][]*maintenance.MaintenanceExclusionPeriod{},
		},
		team.NewAirflowClient(manager),
		&MockTeamkatalogClient{Teams: mockTeamkatalogTeams},
	)
	if err != nil {
		log.Fatalf("setting up api: %v", err)
	}

	server = httptest.NewServer(router)
	code := m.Run()

	server.Close()
	os.Exit(code)
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
	tmpl, err := template.New("").
		Funcs(template.FuncMap{"toArray": toArray}).
		ParseGlob("templates/**/*")
	if err != nil {
		return "", err
	}

	if err := tmpl.ExecuteTemplate(buff, t, values); err != nil {
		return "", err
	}

	dataBytes, err := io.ReadAll(buff)
	if err != nil {
		return "", err
	}

	return string(dataBytes), nil
}

// Need to move this
func toArray(args ...any) []any {
	return args
}
