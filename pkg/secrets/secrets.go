package secrets

import (
	"github.com/navikt/knorten/pkg/k8s"
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
}

func New(manager k8s.Manager, defaultGCPProject, defaultGCPLocation string) *ExternalSecretClient {
	return &ExternalSecretClient{
		manager:            manager,
		defaultGCPProject:  defaultGCPProject,
		defaultGCPLocation: defaultGCPLocation,
	}
}
