package externalsecret

import (
	"time"

	"github.com/external-secrets/external-secrets/apis/externalsecrets/v1beta1"
	"github.com/navikt/knorten/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewExternalSecretManifest(secrets map[string]string, teamID, secretGroup string) *v1beta1.ExternalSecret {
	secretData := []v1beta1.ExternalSecretData{}
	for remoteRef, clusterSecretKey := range secrets {
		secretData = append(secretData, v1beta1.ExternalSecretData{
			SecretKey: clusterSecretKey,
			RemoteRef: v1beta1.ExternalSecretDataRemoteRef{
				Key: remoteRef,
			},
		})
	}

	return &v1beta1.ExternalSecret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ExternalSecret",
			APIVersion: "external-secrets.io/v1beta1",
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
