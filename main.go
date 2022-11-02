package main

import (
	"encoding/json"
	"os"

	"github.com/nais/knorten/pkg/api"
	"github.com/nais/knorten/pkg/auth"
	"github.com/nais/knorten/pkg/database"
	"github.com/sirupsen/logrus"
	//_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func main() {
	log := logrus.New()

	repo, err := database.New("postgres://postgres:postgres@localhost:5432/knorten?sslmode=disable", log.WithField("subsystem", "repo"))
	if err != nil {
		log.WithError(err).Fatal("setting up database")
	}

	bytes, err := os.ReadFile("aad_secret.json")
	if err != nil {
		panic("error reading aad config")
	}

	var oauthConfig auth.OauthConfig
	if err := json.Unmarshal(bytes, &oauthConfig); err != nil {
		panic("unmarshalling aad config json")
	}

	azure := auth.New(&oauthConfig, log.WithField("subfield", "auth"))

	//ctx := context.Background()
	//jhub := helm.NewJupyterhub("nada", "charts/jupyterhub/values.yaml", repo)
	//chartVals, err := jhub.ChartValues(ctx)
	//if err != nil {
	//	panic(err)
	//}
	//
	//fmt.Println(chartVals)

	// kApi := api.New(repo)
	kApi := api.New(repo, azure, log.WithField("subsystem", "api"))
	err = kApi.Run()
	if err != nil {
		return
	}
	// http.ListenAndServe(":8080", kApi.Router)
}
