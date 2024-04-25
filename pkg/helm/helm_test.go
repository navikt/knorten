package helm_test

import (
	"os"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/navikt/knorten/pkg/helm"
)

func TestEstablishEnv(t *testing.T) {
	testCases := []struct {
		name   string
		envs   map[string]string
		expect []string
	}{
		{
			name: "With some envs",
			envs: map[string]string{
				"SOMETHING": "cool",
			},
			expect: []string{
				"SOMETHING=cool",
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			os.Clearenv()
			t.Setenv("TESTING", "env")

			before := os.Environ()

			restoreFn, err := helm.EstablishEnv(tc.envs)
			if err != nil {
				t.Error(err)
			}

			if !slices.Equal(tc.expect, os.Environ()) {
				t.Errorf("establish: expected %v, got %v", tc.expect, os.Environ())
			}

			err = restoreFn()
			if err != nil {
				t.Error(err)
			}

			after := os.Environ()
			if diff := cmp.Diff(before, after); diff != "" {
				t.Errorf("restore: (-before +after)\n%s", diff)
			}
		})
	}
}
