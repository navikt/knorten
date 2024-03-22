package core_test

import (
	"testing"

	"github.com/navikt/knorten/pkg/k8s/core"
	"github.com/sebdah/goldie/v2"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func TestNamespace(t *testing.T) {
	testCases := []struct {
		name      string
		desc      string
		namespace *v1.Namespace
	}{
		{
			name:      "plain-namespace",
			desc:      "Create a plain namespace",
			namespace: core.NewNamespace("test"),
		},
		{
			name: "namespace-with-team-namespace-label",
			desc: "Create a namespace with team namespace label",
			namespace: core.NewNamespace(
				"test",
				core.WithTeamNamespaceLabel(),
			),
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			g := goldie.New(t)

			d, err := yaml.Marshal(tc.namespace)
			if err != nil {
				t.Fatal(err)
			}

			g.Assert(t, tc.name, d)
		})
	}
}

func TestSecret(t *testing.T) {
	testCases := []struct {
		name   string
		desc   string
		secret *v1.Secret
	}{
		{
			name:   "plain-secret",
			desc:   "Create a plain secret",
			secret: core.NewSecret("test", "test-namespace", map[string]string{"key": "value"}),
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			g := goldie.New(t)

			d, err := yaml.Marshal(tc.secret)
			if err != nil {
				t.Fatal(err)
			}

			g.Assert(t, tc.name, d)
		})
	}
}

func TestServiceAccount(t *testing.T) {
	testCases := []struct {
		name string
		desc string
		sa   *v1.ServiceAccount
	}{
		{
			name: "plain-serviceaccount",
			desc: "Create a plain service account",
			sa:   core.NewServiceAccount("test", "test-namespace"),
		},
		{
			name: "serviceaccount-with-annotation",
			desc: "Create a service account with annotation",
			sa: core.NewServiceAccount(
				"test",
				"test-namespace",
				core.WithGKEIAMAccountAnnotation("test-team", "test-gcp-project"),
			),
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			g := goldie.New(t)

			d, err := yaml.Marshal(tc.sa)
			if err != nil {
				t.Fatal(err)
			}

			g.Assert(t, tc.name, d)
		})
	}
}
