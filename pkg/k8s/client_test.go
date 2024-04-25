package k8s_test

import (
	"testing"

	"github.com/navikt/knorten/pkg/k8s"
	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
)

func TestKubeConfigFromREST(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		golden    string
		cfg       *rest.Config
		expect    interface{}
		expectErr bool
	}{
		{
			name:   "Should return kubeconfig",
			golden: "test-config",
			cfg: &rest.Config{
				Host:    "https://localhost:8080",
				APIPath: "/api",
				TLSClientConfig: rest.TLSClientConfig{
					CAData: []byte("ca-data"),
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := k8s.NewKubeConfig(tc.golden)

			err := c.FromREST(tc.cfg)

			if tc.expectErr {
				assert.NoError(t, err)
				assert.Equal(t, tc.expect, err)
			} else {
				assert.NoError(t, err)

				g := goldie.New(t)
				g.Assert(t, tc.golden, c.Contents())
			}
		})
	}
}
