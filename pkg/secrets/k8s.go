package secrets

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/external-secrets/external-secrets/apis/externalsecrets/v1beta1"
	"github.com/navikt/knorten/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (e *ExternalSecretClient) ApplyExternalSecret(ctx context.Context, teamID, secretGroup string) error {
	secrets, err := e.GetTeamSecretGroup(ctx, nil, teamID, secretGroup)
	if err != nil {
		return err
	}

	err = e.manager.ApplyExternalSecret(ctx, createExternalSecretManifest(secrets, teamID, secretGroup))
	if err != nil {
		return fmt.Errorf("applying external secret: %w", err)
	}

	return nil
}

func createExternalSecretManifest(secrets []TeamSecret, teamID, secretGroup string) *v1beta1.ExternalSecret {
	secretData := []v1beta1.ExternalSecretData{}
	for _, secret := range secrets {
		secretName := getSecretNameFromPath(secret.Key)
		secretData = append(secretData, v1beta1.ExternalSecretData{
			SecretKey: secretName,
			RemoteRef: v1beta1.ExternalSecretDataRemoteRef{
				Key: secretName,
			},
		})
	}

	return &v1beta1.ExternalSecret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ExternalSecret",
			APIVersion: "v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretGroup,
			Namespace: k8s.TeamIDToNamespace(teamID),
		},
		Spec: v1beta1.ExternalSecretSpec{
			Data:            secretData,
			RefreshInterval: &metav1.Duration{Duration: 10 * time.Minute},
			Target: v1beta1.ExternalSecretTarget{
				Name: secretGroup,
			},
			SecretStoreRef: v1beta1.SecretStoreRef{
				Kind: v1beta1.ClusterSecretStoreKind,
				Name: "default-gsm-store",
			},
		},
	}
}

func getSecretNameFromPath(secretPath string) string {
	pathParts := strings.Split(secretPath, "/")
	return pathParts[len(pathParts)-1]
}
