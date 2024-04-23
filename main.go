package main

import (
	"context"
	"flag"
	"html/template"
	"io"
	"net"
	"os"
	"time"

	"github.com/navikt/knorten/pkg/gcpapi"
	"github.com/navikt/knorten/pkg/gcpapi/mock"
	"github.com/navikt/knorten/pkg/k8s"
	"google.golang.org/api/iam/v1"

	"github.com/gin-gonic/gin"
	"github.com/navikt/knorten/pkg/api"
	"github.com/navikt/knorten/pkg/api/auth"
	"github.com/navikt/knorten/pkg/api/handlers"
	"github.com/navikt/knorten/pkg/api/middlewares"
	"github.com/navikt/knorten/pkg/api/service"
	"github.com/navikt/knorten/pkg/config"
	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/events"
	"github.com/navikt/knorten/pkg/helm"
	"github.com/navikt/knorten/pkg/imageupdater"
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
	}

	cfg, err := config.NewFileSystemLoader().Load(fileParts.FileName, fileParts.Path, "KNORTEN")
	if err != nil {
		log.WithError(err).Fatal("loading config")
	}

	err = cfg.Validate()
	if err != nil {
		log.WithError(err).Fatal("validating config")
	}

	dbClient, err := database.New(cfg.Postgres.ConnectionString(), cfg.DBEncKey, log.WithField("subsystem", "db"))
	if err != nil {
		log.WithError(err).Fatal("setting up database")
	}

	azureClient, err := auth.NewAzureClient(
		cfg.DryRun,
		cfg.Oauth.ClientID,
		cfg.Oauth.ClientSecret,
		cfg.Oauth.TenantID,
		cfg.Oauth.RedirectURL,
		log.WithField("subsystem", "auth"),
	)
	if err != nil {
		log.WithError(err).Fatal("creating azure client")
	}

	if !cfg.DryRun {
		imageUpdater := imageupdater.NewClient(dbClient, log.WithField("subsystem", "imageupdater"))
		go imageUpdater.Run(imageUpdaterFrequency)
	}

	c, err := k8s.NewClient(cfg.Kubernetes.Context, k8s.DefaultSchemeAdder())
	if err != nil {
		log.WithError(err).Fatal("creating k8s client")
	}

	if cfg.DryRun {
		c.Client = k8s.NewDryRunClient(c.Client)
	}

	ctx := context.Background()

	iamService, err := iam.NewService(ctx)
	if err != nil {
		log.WithError(err).Fatal("creating iam service")
	}

	policyManager := gcpapi.NewServiceAccountPolicyManager(cfg.GCP.Project, iamService)
	fetcher := gcpapi.NewServiceAccountFetcher(cfg.GCP.Project, iamService)

	if cfg.DryRun {
		policyManager = mock.NewServiceAccountPolicyManager(&iam.Policy{}, nil)
		fetcher = mock.NewServiceAccountFetcher(&iam.ServiceAccount{}, nil)
	}

	binder := gcpapi.NewServiceAccountPolicyBinder(cfg.GCP.Project, policyManager)
	checker := gcpapi.NewServiceAccountChecker(cfg.GCP.Project, fetcher)

	errOut := io.Discard
	if cfg.Debug {
		errOut = os.Stderr
	}

	out := io.Discard
	if cfg.Debug {
		out = os.Stdout
	}

	helmConfig := &helm.Config{
		Debug:            cfg.Debug,
		DryRun:           cfg.DryRun,
		Err:              errOut,
		KubeContext:      cfg.Kubernetes.Context,
		Out:              out,
		RepositoryConfig: cfg.Helm.RepositoryConfig,
	}

	helmClient, err := helm.NewClient(helmConfig, dbClient)
	if err != nil {
		log.WithError(err).Fatal("creating helm client")
	}

	eventHandler, err := events.NewHandler(
		ctx,
		dbClient,
		azureClient,
		k8s.NewManager(c),
		binder,
		checker,
		helmClient,
		cfg.GCP.Project,
		cfg.GCP.Region,
		cfg.GCP.Zone,
		cfg.Helm.AirflowChartVersion,
		cfg.Helm.JupyterChartVersion,
		cfg.TopLevelDomain,
		cfg.DryRun,
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
		cfg.AdminGroupID,
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

	err = api.New(router, dbClient, azureClient, log.WithField("subsystem", "api"), cfg.DryRun, cfg.GCP.Project, cfg.GCP.Zone, cfg.TopLevelDomain)
	if err != nil {
		log.WithError(err).Fatal("creating api")
		return
	}

	err = router.Run(net.JoinHostPort(cfg.Server.Hostname, cfg.Server.Port))
	if err != nil {
		log.WithError(err).Fatal("running api")

		return
	}
}

// Need to move this
func toArray(args ...any) []any {
	return args
}
