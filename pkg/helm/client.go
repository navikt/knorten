package helm

import (
	"context"
	"fmt"
	"log"

	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
)

type HelmApplication interface {
	Chart(ctx context.Context) (*chart.Chart, error)
}

type HelmClient struct {
	log *logrus.Entry
}

func New(log *logrus.Entry) *HelmClient {
	return &HelmClient{
		log: log,
	}
}

func (h *HelmClient) InstallOrUpgrade(releaseName, namespace string, app HelmApplication) {
	hChart, err := app.Chart(context.Background())
	if err != nil {
		h.log.WithError(err).Error("install or upgrading release %v", releaseName)
	}

	settings := cli.New()
	settings.SetNamespace(namespace)
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), "secret", log.Printf); err != nil {
		log.Printf("%+v", err)
		h.log.WithError(err).Error("install or upgrading release %v", releaseName)
		return
	}

	listClient := action.NewList(actionConfig)
	listClient.Deployed = true
	results, err := listClient.Run()
	if err != nil {
		h.log.WithError(err).Error("install or upgrading release %v", releaseName)
		return
	}

	exists := false
	for _, rel := range results {
		if rel.Name == releaseName {
			exists = true
		}
	}

	var release *release.Release
	if !exists {
		fmt.Println("Installing release")
		installClient := action.NewInstall(actionConfig)
		installClient.Namespace = namespace
		installClient.CreateNamespace = true
		installClient.ReleaseName = releaseName

		release, err = installClient.Run(hChart, hChart.Values)
		if err != nil {
			h.log.WithError(err).Error("install or upgrading release %v", releaseName)
			return
		}
	} else {
		fmt.Println("Upgrading existing release", releaseName)
		upgradeClient := action.NewUpgrade(actionConfig)
		upgradeClient.Namespace = namespace

		release, err = upgradeClient.Run(releaseName, hChart, hChart.Values)
		if err != nil {
			h.log.WithError(err).Error("install or upgrading release %v", releaseName)
			return
		}
	}

	fmt.Println(release.Info)
}
