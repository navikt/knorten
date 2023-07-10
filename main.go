package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/nais/knorten/pkg/api"
	"github.com/nais/knorten/pkg/auth"
	"github.com/nais/knorten/pkg/chart"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/database/crypto"
	"github.com/nais/knorten/pkg/events"
	"github.com/nais/knorten/pkg/google"
	"github.com/nais/knorten/pkg/imageupdater"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/nais/knorten/pkg/team"
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
	KnelmImage          string
	AirflowChartVersion string
	JupyterChartVersion string
	VMNetworkConfig     string
	AdminGroup          string
	SessionKey          string
}

func main() {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})

	cfg := Config{}
	flag.StringVar(&cfg.Hostname, "hostname", os.Getenv("HOSTNAME"), "Hostname the application is served from")
	flag.StringVar(&cfg.ClientID, "oauth2-client-id", os.Getenv("AZURE_APP_CLIENT_ID"), "Client ID for azure app")
	flag.StringVar(&cfg.ClientSecret, "oauth2-client-secret", os.Getenv("AZURE_APP_CLIENT_SECRET"), "Client secret for azure app")
	flag.StringVar(&cfg.TenantID, "oauth2-tenant-id", os.Getenv("AZURE_APP_TENANT_ID"), "OAuth2 tenant ID")
	flag.StringVar(&cfg.DBConnString, "db-conn-string", os.Getenv("DB_CONN_STRING"), "Database connection string")
	flag.StringVar(&cfg.DBEncKey, "db-enc-key", os.Getenv("DB_ENC_KEY"), "Chart value encryption key")
	flag.BoolVar(&cfg.DryRun, "dry-run", false, "Don't run external commands")
	flag.BoolVar(&cfg.InCluster, "in-cluster", true, "In cluster configuration for go client")
	flag.StringVar(&cfg.GCPProject, "project", os.Getenv("GCP_PROJECT"), "GCP project")
	flag.StringVar(&cfg.GCPRegion, "region", os.Getenv("GCP_REGION"), "GCP region")
	flag.StringVar(&cfg.KnelmImage, "knelm-image", os.Getenv("KNELM_IMAGE"), "Knelm image")
	flag.StringVar(&cfg.AirflowChartVersion, "airflow-chart-version", os.Getenv("AIRFLOW_CHART_VERSION"), "The chart version for airflow")
	flag.StringVar(&cfg.JupyterChartVersion, "jupyter-chart-version", os.Getenv("JUPYTER_CHART_VERSION"), "The chart version for jupyter")
	flag.StringVar(&cfg.AdminGroup, "admin-group", os.Getenv("ADMIN_GROUP"), "Email of admin group used to authenticate Knorten administrators")
	flag.StringVar(&cfg.VMNetworkConfig, "vm-network-config", os.Getenv("VM_NETWORK_CONFIG"), "Network configuration for compute instances created by knorten")
	flag.StringVar(&cfg.SessionKey, "session-key", os.Getenv("SESSION_KEY"), "The session key for Knorten")
	flag.Parse()

	querier, dbClient, err := database.New(fmt.Sprintf("%v?sslmode=disable", cfg.DBConnString), log.WithField("subsystem", "db"))
	if err != nil {
		log.WithError(err).Fatal("setting up database")
		return
	}

	authClient := auth.New(cfg.DryRun, cfg.ClientID, cfg.ClientSecret, cfg.TenantID, cfg.Hostname, log.WithField("subsystem", "auth"))

	googleClient := google.New(dbClient, cfg.GCPProject, cfg.GCPRegion, cfg.VMNetworkConfig, cfg.DryRun, log.WithField("subsystem", "google"))

	cryptClient := crypto.New(cfg.DBEncKey)

	k8sClient, err := k8s.New(cryptClient, dbClient, cfg.DryRun, cfg.InCluster, cfg.GCPProject, cfg.GCPRegion, cfg.KnelmImage, cfg.AirflowChartVersion, cfg.JupyterChartVersion, log.WithField("subsystem", "k8sClient"))
	if err != nil {
		log.WithError(err).Fatal("creating k8s client")
		return
	}

	if !cfg.DryRun {
		imageUpdater := imageupdater.New(dbClient, googleClient, k8sClient, authClient, cryptClient, cfg.JupyterChartVersion, cfg.AirflowChartVersion, log.WithField("subsystem", "imageupdater"))
		go imageUpdater.Run(imageUpdaterFrequency)
	}

	chartClient, err := chart.New(dbClient, googleClient, k8sClient, authClient, cryptClient, cfg.AirflowChartVersion, cfg.JupyterChartVersion, log.WithField("subsystem", "chartClient"))
	if err != nil {
		log.WithError(err).Fatal("creating chart client")
		return
	}

	teamClient := team.NewClient(dbClient, googleClient, k8sClient, chartClient, authClient, cfg.DryRun, log.WithField("subsystem", "teamClient"))

	go events.Start(context.Background(), querier, teamClient, log.WithField("subsystem", "events"))

	router, err := api.New(dbClient, authClient, googleClient, k8sClient, cryptClient, chartClient, teamClient, cfg.DryRun, cfg.AirflowChartVersion, cfg.JupyterChartVersion, cfg.SessionKey, cfg.AdminGroup, log.WithField("subsystem", "api"))
	if err != nil {
		log.WithError(err).Fatal("creating api")
		return
	}

	err = api.Run(router, cfg.InCluster)
	if err != nil {
		return
	}
}
