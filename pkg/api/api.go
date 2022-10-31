package api

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/nais/knorten/pkg/database"
	"github.com/nais/knorten/pkg/helm"
)

type API struct {
	Router     *chi.Mux
	helmClient helm.HelmClient
	repo       *database.Repo
}

func New() *API {
	r := chi.NewRouter()
	return &API{
		Router: r,
	}
}

func (a *API) AddHandlers() {
	a.Router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("knorten"))
	})

	a.Router.Post("/", func(w http.ResponseWriter, r *http.Request) {
		releaseName := "team-nada-jupyterhub"
		team := "team-nada"
		jupyterhub := helm.NewJupyterhub("nada", "charts/jupyterhub/values.yaml", a.repo)
		a.helmClient.InstallOrUpgrade(releaseName, team, jupyterhub)
	})
}
