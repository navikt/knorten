package main

import (
	"fmt"
	"log"
	"os"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func main() {
	// Hello
	// 1. Rulle ut helm-chart med values
	settings := cli.New()
	settings.SetNamespace("team-nada")

	actionConfig := new(action.Configuration)
	// You can pass an empty string instead of settings.Namespace() to list
	// all namespaces
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), "secret", log.Printf); err != nil {
		log.Printf("%+v", err)
		os.Exit(1)
	}

	client := action.NewList(actionConfig)
	// Only list deployed
	client.Deployed = true
	results, err := client.Run()
	if err != nil {
		log.Printf("%+v", err)
		os.Exit(1)
	}

	for _, rel := range results {
		log.Printf("%+v", rel.Name)
	}

	destDir := "/tmp"
	chartName := "jupyterhub"
	version := "0.11.1"
	pullClient := action.NewPullWithOpts(action.WithConfig(actionConfig))
	pullClient.RepoURL = "https://jupyterhub.github.io/helm-chart"
	pullClient.Settings = settings
	pullClient.DestDir = destDir
	pullClient.Version = version
	_, err = pullClient.Run(chartName)
	if err != nil {
		log.Panicf("failed to pull chart: %v, error: %v", chartName, err)
	}

	chart, err := loader.Load(fmt.Sprintf("%v/%v-%v.tgz", destDir, chartName, version))
	upgradeClient := action.NewUpgrade(actionConfig)
	upgradeClient.Namespace = "team-nada"
	upgradeClient.Install = true
	if err != nil {
		panic(err)
	}

	proxy := chart.Values["proxy"].(map[string]interface{})
	proxy["secretToken"] = ""
	chart.Values["proxy"] = proxy

	release, err := upgradeClient.Run("jlab", chart, chart.Values)
	if err != nil {
		panic(err)
	}

	fmt.Println(release.Info)
}
