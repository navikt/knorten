package main

import (
	"log"
	"os"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func main() {
	// Hello
	// 1. Rulle ut helm-chart med values
	//airflowEntry := &helmRepo.Entry{
	//	Name: "Airflow",
	//	URL:  "",
	//}
	//airflowProviders := getter.Providers{
	//	{
	//		Schemes: nil,
	//		New:     nil,
	//	},
	//}
	//
	//chartRepository, err := helmRepo.NewChartRepository(airflowEntry, airflowProviders)
	//if err != nil {
	//	panic(err)
	//}
	//
	//airflowConfig := helmAction.Configuration{
	//	RESTClientGetter: nil,
	//	Releases:         nil,
	//	KubeClient:       nil,
	//	RegistryClient:   nil,
	//	Capabilities:     nil,
	//	Log:              nil,
	//}
	//helmAction.NewUpgrade()

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
}
