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
	return nil
	secrets, err := e.GetTeamSecretGroup(ctx, nil, teamID, secretGroup)
	if err != nil {
		return err
	}

	err = e.manager.ApplyExternalSecret(ctx, createExternalSecretManifest(secrets, teamID, secretGroup))
	if err != nil {
		return fmt.Errorf("applying external secret %v for team %v: %w", secretGroup, teamID, err)
	}

	return nil
}

func (e *ExternalSecretClient) DeleteExternalSecret(ctx context.Context, teamID, secretGroup string) error {
	if err := e.deleteTeamSecretGroup(ctx, nil, teamID, secretGroup); err != nil {
		return fmt.Errorf("deleting gsm secrets group %v for team %v: %w", secretGroup, teamID, err)
	}

	if err := e.manager.DeleteExternalSecret(ctx, secretGroup, k8s.TeamIDToNamespace(teamID)); err != nil {
		return fmt.Errorf("deleting external secret %v for team %v: %w", secretGroup, teamID, err)
	}

	return nil
}

func createExternalSecretManifest(secrets []TeamSecret, teamID, secretGroup string) *v1beta1.ExternalSecret {
	secretData := []v1beta1.ExternalSecretData{}
	for _, secret := range secrets {
		remoteSecretKey, k8sSecretKey := getSecretNameFromPath(secret.Key)
		secretData = append(secretData, v1beta1.ExternalSecretData{
			SecretKey: k8sSecretKey,
			RemoteRef: v1beta1.ExternalSecretDataRemoteRef{
				Key: remoteSecretKey,
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

func getSecretNameFromPath(secretPath string) (string, string) {
	pathParts := strings.Split(secretPath, "/")
	secretNameParts := strings.Split(pathParts[len(pathParts)-1], "-")
	return pathParts[len(pathParts)-1], secretNameParts[len(secretNameParts)-1]
}
