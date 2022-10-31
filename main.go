package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"text/template"

	"github.com/knadh/koanf/maps"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

type Vars struct {
	GlobalVars
	TeamVars
}

type GlobalVars struct {
	ImageName string
	ImageTag  string
}

type TeamVars struct {
	ProxyToken string
}

func getChart(chartName, version, repo string, actionConfig *action.Configuration, settings *cli.EnvSettings) (*chart.Chart, error) {
	destDir := "/tmp"
	pullClient := action.NewPullWithOpts(action.WithConfig(actionConfig))
	pullClient.RepoURL = repo
	pullClient.Settings = settings
	pullClient.DestDir = destDir
	pullClient.Version = version
	_, err := pullClient.Run(chartName)
	if err != nil {
		log.Panicf("failed to pull chart: %v, error: %v", chartName, err)
	}

	return loader.Load(fmt.Sprintf("%v/%v-%v.tgz", destDir, chartName, version))
}

func main() {
	releaseName := "jlab"
	namespace := "team-nada"
	settings := cli.New()
	settings.SetNamespace(namespace)
	log := logrus.New()

	// helmClient := helm.New(logrus.NewEntry(log).WithField("subsystem", "helmClient"))

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), "secret", log.Printf); err != nil {
		log.Printf("%+v", err)
		os.Exit(1)
	}

	// Prepare values
	dataBytes, err := ioutil.ReadFile("./values.yaml")
	if err != nil {
		panic(err)
	}

	// From database
	globalVars := GlobalVars{
		ImageName: "image",
		ImageTag:  "tag",
	}

	// From database
	teamVars := TeamVars{
		ProxyToken: "",
	}

	// All vars combined
	vars := Vars{
		globalVars,
		teamVars,
	}

	tmpl, err := template.New("template").Parse(string(dataBytes))
	if err != nil {
		panic(err)
	}

	buffer := &bytes.Buffer{}
	if err := tmpl.Execute(buffer, vars); err != nil {
		panic(err)
	}

	var values map[string]interface{}
	if err := yaml.Unmarshal(buffer.Bytes(), &values); err != nil {
		panic(err)
	}

	chart, err := getChart("jupyterhub", "0.11.1", "https://jupyterhub.github.io/helm-chart", actionConfig, settings)
	if err != nil {
		panic(err)
	}

	maps.IntfaceKeysToStrings(values)
	maps.IntfaceKeysToStrings(chart.Values)
	maps.Merge(values, chart.Values)

	// Deploy
	client := action.NewList(actionConfig)
	client.Deployed = true
	results, err := client.Run()
	if err != nil {
		log.Printf("%+v", err)
		os.Exit(1)
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

		release, err = installClient.Run(chart, chart.Values)
		if err != nil {
			panic(err)
		}
	} else {
		fmt.Println("Upgrading existing release", releaseName)
		upgradeClient := action.NewUpgrade(actionConfig)
		upgradeClient.Namespace = namespace

		release, err = upgradeClient.Run(releaseName, chart, chart.Values)
		if err != nil {
			panic(err)
		}
	}

	fmt.Println(release.Info)
}
