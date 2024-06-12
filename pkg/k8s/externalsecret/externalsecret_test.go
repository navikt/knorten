package externalsecret_test

import (
	"testing"

	"github.com/external-secrets/external-secrets/apis/externalsecrets/v1beta1"
	"github.com/navikt/knorten/pkg/k8s/externalsecret"
	"github.com/sebdah/goldie/v2"
	"gopkg.in/yaml.v2"
)

func TestExternalSecret(t *testing.T) {
	testCases := []struct {
		name           string
		desc           string
		externalSecret *v1beta1.ExternalSecret
	}{
		{
			name: "plain-external-secret",
			desc: "Create a plain namespace",
			externalSecret: externalsecret.NewExternalSecretManifest(
				map[string]string{
					"remoteRef":  "clusterSecretKey",
					"remoteRef2": "clusterSecretKey2",
				},
				"test-1234",
				"group",
			),
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			g := goldie.New(t)

			d, err := yaml.Marshal(tc.externalSecret)
			if err != nil {
				t.Fatal(err)
			}

			g.Assert(t, tc.name, d)
		})
	}
}
