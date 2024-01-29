package main

import (
	"context"
	"flag"
	"fmt"
	"html/template"
	"time"

	"github.com/nais/knorten/pkg/api/service"

	"github.com/nais/knorten/pkg/api/handlers"

	"github.com/nais/knorten/pkg/api/middlewares"

	"github.com/gin-gonic/gin"

	"github.com/spf13/afero"

	"github.com/nais/knorten/pkg/config"

	"github.com/nais/knorten/pkg/api"
	"github.com/nais/knorten/pkg/api/auth"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/events"
	"github.com/nais/knorten/pkg/helm"
	"github.com/nais/knorten/pkg/imageupdater"
	"github.com/sirupsen/logrus"
)

const (
	imageUpdaterFrequency = 24 * time.Hour
)

var configFilePath = flag.String("config", "config.yaml", "path to config file")

func main() {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})

	flag.Parse()

	fileParts, err := config.ProcessConfigPath(*configFilePath)
	if err != nil {
		log.WithError(err).Fatal("processing config path")

		return
	}

	cfg, err := config.NewFileSystemLoader(afero.NewOsFs()).Load(fileParts.FileName, fileParts.Path, "")
	if err != nil {
		log.WithError(err).Fatal("loading config")

		return
	}

	err = cfg.Validate()
	if err != nil {
		log.WithError(err).Fatal("validating config")

		return
	}

	dbClient, err := database.New(cfg.Postgres.ConnectionString(), cfg.DBEncKey, log.WithField("subsystem", "db"))
	if err != nil {
		log.WithError(err).Fatal("setting up database")
		return
	}

	azureClient, err := auth.NewAzureClient(
		cfg.DryRun,
		cfg.Oauth.ClientID,
		cfg.Oauth.ClientSecret,
		cfg.Oauth.TenantID,
		log.WithField("subsystem", "auth"),
	)
	if err != nil {
		log.WithError(err).Fatal("creating azure client")
		return
	}

	if !cfg.DryRun {
		imageUpdater := imageupdater.NewClient(dbClient, log.WithField("subsystem", "imageupdater"))
		go imageUpdater.Run(imageUpdaterFrequency)

		if err := helm.UpdateHelmRepositories(); err != nil {
			log.WithError(err).Fatal("updating helm repositories")
		}
	}

	eventHandler, err := events.NewHandler(
		context.Background(),
		dbClient,
		azureClient,
		cfg.GCP.Project,
		cfg.GCP.Region,
		cfg.GCP.Zone,
		cfg.Helm.AirflowChartVersion,
		cfg.Helm.JupyterChartVersion,
		cfg.DryRun,
		cfg.InCluster,
		log.WithField("subsystem", "events"),
	)
	if err != nil {
		log.WithError(err).Fatal("starting event watcher")
		return
	}
	eventHandler.Run(10 * time.Second)

	router := gin.New()

	session, err := dbClient.NewSessionStore(cfg.SessionKey)
	if err != nil {
		log.WithError(err).Fatal("creating session store")

		return
	}

	authService := service.NewAuthService(
		dbClient,
		cfg.AdminGroup,
		1*time.Hour,
		32,
		azureClient,
	)

	authHandler := handlers.NewAuthHandler(
		authService,
		cfg.LoginPage,
		cfg.Cookies,
		log.WithField("subsystem", "auth"),
		dbClient,
	)

	router.Use(session)
	router.Static("/assets", "./assets")
	router.FuncMap = template.FuncMap{
		"toArray": toArray,
	}
	router.LoadHTMLGlob("templates/**/*")
	router.Use(middlewares.SetSessionStatus(log.WithField("subsystem", "status_middleware"), cfg.Cookies.Session.Name, dbClient))
	router.GET("/", handlers.IndexHandler)
	router.GET("/oauth2/login", authHandler.LoginHandler(cfg.DryRun))
	router.GET("/oauth2/callback", authHandler.CallbackHandler())
	router.GET("/oauth2/logout", authHandler.LogoutHandler())
	router.Use(middlewares.Authenticate(
		log.WithField("subsystem", "authentication"),
		dbClient,
		azureClient,
		cfg.DryRun,
	))

	err = api.New(router, dbClient, azureClient, log.WithField("subsystem", "api"), api.Config{
		DryRun:          cfg.DryRun,
		AdminGroupEmail: cfg.AdminGroup,
		GCPProject:      cfg.GCP.Project,
		GCPZone:         cfg.GCP.Zone,
	})
	if err != nil {
		log.WithError(err).Fatal("creating api")
		return
	}

	err = router.Run(fmt.Sprintf("%s:%d", cfg.Server.Hostname, cfg.Server.Port))
	if err != nil {
		log.WithError(err).Fatal("running api")

		return
	}
}

// Need to move this
func toArray(args ...any) []any {
	return args
}
