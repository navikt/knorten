package secrets

import (
	"context"

	"github.com/navikt/knorten/pkg/database"
	"github.com/navikt/knorten/pkg/k8s"
	"github.com/sirupsen/logrus"
)

const (
	knadaSecretPrefix      = "knada"
	externalSecretLabelKey = "external-secret"
	teamIDLabelKey         = "team-id"
	secretGroupKey         = "secret-group"
)

type EventData struct {
	TeamID      string `json:"teamID"`
	SecretGroup string `json:"secretGroup"`
}

type SecretGroup struct {
	GCPProject string       `json:"gcpProject"`
	Secrets    []TeamSecret `json:"secrets"`
}

type TeamSecret struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Name  string `json:"name"`
}

type ExternalSecretClient struct {
	manager            k8s.Manager
	defaultGCPProject  string
	defaultGCPLocation string
	log                *logrus.Entry
}

func New(ctx context.Context, repo *database.Repo, manager k8s.Manager, defaultGCPProject, defaultGCPLocation string, log *logrus.Entry) *ExternalSecretClient {
	return &ExternalSecretClient{
		manager:            manager,
		defaultGCPProject:  defaultGCPProject,
		defaultGCPLocation: defaultGCPLocation,
		log:                log,
	}
}
