package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/nais/knorten/pkg/api"
	"github.com/nais/knorten/pkg/auth"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/helm"
	"github.com/sirupsen/logrus"
)

type Config struct {
	auth.OauthConfig

	DBConnString string
	DryRun       bool
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
	flag.Parse()

	repo, err := database.New(fmt.Sprintf("%v?sslmode=disable", cfg.DBConnString), log.WithField("subsystem", "repo"))
	if err != nil {
		log.WithError(err).Fatal("setting up database")
	}

	helmClient := helm.New(repo, log.WithField("subsystem", "helmClient"), cfg.DryRun)

	azure := auth.New(cfg.ClientID, cfg.ClientSecret, cfg.TenantID, cfg.Hostname, log.WithField("subfield", "auth"))

	kApi := api.New(repo, azure, helmClient, log.WithField("subsystem", "api"), cfg.DryRun)
	err = kApi.Run()
	if err != nil {
		return
	}
}
