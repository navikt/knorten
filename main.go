package main

import (
	"flag"
	"os"
	"time"

	"github.com/nais/knorten/pkg/api/auth"
	"github.com/sirupsen/logrus"
)

const (
	imageUpdaterFrequency = 24 * time.Hour
)

type Config struct {
	auth.OauthConfig

	DBConnString        string
	DBEncKey            string
	DryRun              bool
	InCluster           bool
	GCPProject          string
	GCPRegion           string
	GCPZone             string
	AirflowChartVersion string
	JupyterChartVersion string
	AdminGroup          string
	SessionKey          string
}

func main() {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})

	cfg := Config{}
	flag.StringVar(&cfg.ClientID, "oauth2-client-id", os.Getenv("AZURE_APP_CLIENT_ID"), "Client ID for azure app")
	flag.StringVar(&cfg.ClientSecret, "oauth2-client-secret", os.Getenv("AZURE_APP_CLIENT_SECRET"), "Client secret for azure app")
	flag.StringVar(&cfg.TenantID, "oauth2-tenant-id", os.Getenv("AZURE_APP_TENANT_ID"), "OAuth2 tenant ID")
	flag.StringVar(&cfg.DBConnString, "db-conn-string", os.Getenv("DB_CONN_STRING"), "Database connection string")
	flag.StringVar(&cfg.DBEncKey, "db-enc-key", os.Getenv("DB_ENC_KEY"), "Chart value encryption key")
	flag.BoolVar(&cfg.DryRun, "dry-run", false, "Don't run external commands")
	flag.BoolVar(&cfg.InCluster, "in-cluster", true, "In cluster configuration for go client")
	flag.StringVar(&cfg.GCPProject, "project", os.Getenv("GCP_PROJECT"), "GCP project")
	flag.StringVar(&cfg.GCPRegion, "region", os.Getenv("GCP_REGION"), "GCP region")
	flag.StringVar(&cfg.GCPZone, "zone", os.Getenv("GCP_ZONE"), "GCP zone")
	flag.StringVar(&cfg.AirflowChartVersion, "airflow-chart-version", os.Getenv("AIRFLOW_CHART_VERSION"), "The chart version for airflow")
	flag.StringVar(&cfg.JupyterChartVersion, "jupyter-chart-version", os.Getenv("JUPYTER_CHART_VERSION"), "The chart version for jupyter")
	flag.StringVar(&cfg.AdminGroup, "admin-group", os.Getenv("ADMIN_GROUP"), "Email of admin group used to authenticate Knorten administrators")
	flag.StringVar(&cfg.SessionKey, "session-key", os.Getenv("SESSION_KEY"), "The session key for Knorten")
	flag.Parse()

	// dbClient, err := database.New(fmt.Sprintf("%v?sslmode=disable", cfg.DBConnString), cfg.DBEncKey, log.WithField("subsystem", "db"))
	// if err != nil {
	// 	log.WithError(err).Fatal("setting up database")
	// 	return
	// }

	// c := context.Background()
	// ctx, _ := context.WithTimeout(c, time.Second*5)
	// if err != nil {
	// 	log.Error(err)
	// }
	// if err := helm.InstallOrUpgrade(ctx, false, "airflow", "team-knada-hyka", "knada-hyka", "airflow", "apache-airflow", "1.10.0", gensql.ChartTypeAirflow, dbClient); err != nil {
	// 	log.Error(err)
	// }

	// azureClient, err := auth.NewAzureClient(cfg.DryRun, cfg.ClientID, cfg.ClientSecret, cfg.TenantID, log.WithField("subsystem", "auth"))
	// if err != nil {
	// 	log.WithError(err).Fatal("creating azure client")
	// 	return
	// }

	// if !cfg.DryRun {
	// 	imageUpdater := imageupdater.NewClient(dbClient, log.WithField("subsystem", "imageupdater"))
	// 	go imageUpdater.Run(imageUpdaterFrequency)

	// 	if err := helm.UpdateHelmRepositories(); err != nil {
	// 		log.WithError(err).Fatal("updating helm repositories")
	// 	}
	// }

	// eventHandler, err := events.NewHandler(context.Background(), dbClient, azureClient, cfg.GCPProject, cfg.GCPRegion, cfg.GCPZone, cfg.AirflowChartVersion, cfg.JupyterChartVersion, cfg.DryRun, cfg.InCluster, log.WithField("subsystem", "events"))
	// if err != nil {
	// 	log.WithError(err).Fatal("starting event watcher")
	// 	return
	// }
	// eventHandler.Run(10 * time.Second)

	// router, err := api.New(dbClient, azureClient, cfg.DryRun, cfg.SessionKey, cfg.AdminGroup, cfg.GCPProject, cfg.GCPZone, log.WithField("subsystem", "api"))
	// if err != nil {
	// 	log.WithError(err).Fatal("creating api")
	// 	return
	// }

	// err = api.Run(router, cfg.InCluster)
	// if err != nil {
	// 	return
	// }
}
