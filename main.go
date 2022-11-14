package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/nais/knorten/pkg/api"
	"github.com/nais/knorten/pkg/auth"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/google"
	"github.com/nais/knorten/pkg/helm"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/sirupsen/logrus"
)

type Config struct {
	auth.OauthConfig

	DBConnString string
	DryRun       bool
	InCluster    bool
}

func main() {
	log := logrus.New()
	cfg := Config{}

	flag.StringVar(&cfg.Hostname, "hostname", os.Getenv("HOSTNAME"), "Hostname the application is served from")
	flag.StringVar(&cfg.ClientID, "oauth2-client-id", os.Getenv("AZURE_APP_CLIENT_ID"), "Client ID for azure app")
	flag.StringVar(&cfg.ClientSecret, "oauth2-client-secret", os.Getenv("AZURE_APP_CLIENT_SECRET"), "Client secret for azure app")
	flag.StringVar(&cfg.TenantID, "oauth2-tenant-id", os.Getenv("AZURE_APP_TENANT_ID"), "OAuth2 tenant ID")
	flag.StringVar(&cfg.DBConnString, "db-conn-string", os.Getenv("DB_CONN_STRING"), "Database connection string")
	flag.BoolVar(&cfg.DryRun, "dry-run", false, "Don't run Helm commands")
	flag.BoolVar(&cfg.InCluster, "in-cluster", true, "In cluster configuration for go client")
	flag.Parse()

	repo, err := database.New(fmt.Sprintf("%v?sslmode=disable", cfg.DBConnString), log.WithField("subsystem", "repo"))
	if err != nil {
		log.WithError(err).Fatal("setting up database")
		return
	}

	azure := auth.New(cfg.ClientID, cfg.ClientSecret, cfg.TenantID, cfg.Hostname, log.WithField("subfield", "auth"))

	googleClient := google.New(cfg.DryRun)

	helmClient, err := helm.New(repo, log.WithField("subsystem", "helmClient"), cfg.DryRun, cfg.InCluster)
	if err != nil {
		log.WithError(err).Fatal("setting up helm client")
		return
	}

	k8sClient, err := k8s.New(cfg.DryRun, cfg.InCluster)
	if err != nil {
		log.WithError(err).Fatal("creating k8s client")
		return
	}

	kApi, err := api.New(repo, azure, helmClient, googleClient, k8sClient, log.WithField("subsystem", "api"))
	if err != nil {
		log.WithError(err).Fatal("creating api")
		return
	}

	err = kApi.Run()
	if err != nil {
		return
	}
}
