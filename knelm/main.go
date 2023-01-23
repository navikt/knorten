package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/nais/knorten/pkg/database/crypto"
	"github.com/nais/knorten/pkg/helm"
	"github.com/nais/knorten/pkg/k8s"
	"github.com/sirupsen/logrus"
)

type Config struct {
	DecryptKey   string
	Values       string
	ReleaseName  string
	Action       string
	TeamID       string
	ChartVersion string
	KnortenURL   string
}

func main() {
	log := logrus.New()

	cfg := Config{}
	flag.StringVar(&cfg.DecryptKey, "decrypt-key", os.Getenv("DECRYPT_KEY"), "Decrypt key for helm values passed by knorten")
	flag.StringVar(&cfg.Values, "values", os.Getenv("HELM_VALUES"), "Encrypted helm values")
	flag.StringVar(&cfg.ReleaseName, "releasename", os.Getenv("HELM_RELEASE_NAME"), "Helm release name")
	flag.StringVar(&cfg.TeamID, "team", os.Getenv("TEAM_ID"), "Team id for helm release")
	flag.StringVar(&cfg.Action, "action", os.Getenv("HELM_ACTION"), "Helm action")
	flag.StringVar(&cfg.KnortenURL, "url", os.Getenv("KNORTEN_URL"), "URL for knorten")
	flag.StringVar(&cfg.ChartVersion, "chart-version", os.Getenv("CHART_VERSION"), "Version for helm chart")
	flag.Parse()

	helmClient, err := helm.New(log.WithField("subsystem", "helm"))
	if err != nil {
		log.WithError(err).Error("init helm client")
		os.Exit(1)
	}

	switch cfg.Action {
	case string(k8s.InstallOrUpgrade):
		if err := installOrUpgrade(cfg, helmClient); err != nil {
			log.WithError(err).Error("install or upgrade")
			os.Exit(1)
		}
	case string(k8s.Uninstall):
		if err := helmClient.Uninstall(cfg.ReleaseName, k8s.NameToNamespace(cfg.TeamID)); err != nil {
			log.WithError(err).Error("uninstall")
			os.Exit(1)
		}
	default:
		log.Errorf("helm action %v is not valid", cfg.Action)
		os.Exit(1)
	}
}

func installOrUpgrade(cfg Config, helmClient *helm.Client) error {
	cryptoClient := crypto.New(cfg.DecryptKey)
	decryptedValues, err := cryptoClient.DecryptValue(cfg.Values)
	if err != nil {
		return err
	}

	dataBytes, err := base64.StdEncoding.DecodeString(decryptedValues)
	if err != nil {
		return err
	}

	values := map[string]any{}
	if err := json.Unmarshal(dataBytes, &values); err != nil {
		return err
	}

	if err := helmClient.InstallOrUpgrade(cfg.ReleaseName, cfg.ChartVersion, k8s.NameToNamespace(cfg.TeamID), values); err != nil {
		return err
	}

	res, err := http.DefaultClient.Post(fmt.Sprintf("%v/%v/%v", cfg.KnortenURL, cfg.TeamID, helm.ReleaseNameToChartType(cfg.ReleaseName)), "application/json", nil)
	if err != nil {
		return err
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("updating knorten after upgrade returned status code %v (should be 200 ok)", res.StatusCode)
	}

	return nil
}
