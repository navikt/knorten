package teamsecrets

import (
	"context"
	"fmt"
	"strings"

	"github.com/navikt/knorten/pkg/k8s"
	"github.com/navikt/knorten/pkg/k8s/externalsecret"
)

func (e *TeamSecretClient) ApplyExternalSecret(ctx context.Context, teamID, secretGroup string) error {
	gsmSecrets, err := e.GetTeamSecretGroup(ctx, nil, teamID, secretGroup)
	if err != nil {
		return err
	}

	secrets := map[string]string{}
	for _, s := range gsmSecrets {
		remoteRef, clusterSecretKey := remoteRefAndSecretKeyFromGSMPath(s.Key)
		secrets[remoteRef] = clusterSecretKey
	}

	err = e.manager.ApplyExternalSecret(ctx, externalsecret.NewExternalSecretManifest(secrets, teamID, secretGroup))
	if err != nil {
		return fmt.Errorf("applying external secret %v for team %v: %w", secretGroup, teamID, err)
	}

	return nil
}

func (e *TeamSecretClient) DeleteExternalSecret(ctx context.Context, teamID, secretGroup string) error {
	if err := e.manager.DeleteExternalSecret(ctx, secretGroup, k8s.TeamIDToNamespace(teamID)); err != nil {
		return fmt.Errorf("deleting external secret %v for team %v: %w", secretGroup, teamID, err)
	}

	return nil
}

func remoteRefAndSecretKeyFromGSMPath(secretPath string) (string, string) {
	pathParts := strings.Split(secretPath, "/")
	secretNameParts := strings.Split(pathParts[len(pathParts)-1], "-")
	return pathParts[len(pathParts)-1], secretNameParts[len(secretNameParts)-1]
}
