package networking_test

import (
	"testing"

	"github.com/navikt/knorten/pkg/k8s/networking"
	"github.com/sebdah/goldie/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	v1b1 "sigs.k8s.io/gateway-api/apis/v1beta1"
	"sigs.k8s.io/yaml"

	netv1 "k8s.io/api/networking/v1"
)

func TestHTTPRoute(t *testing.T) {
	testCases := []struct {
		name  string
		desc  string
		route *v1b1.HTTPRoute
	}{
		{
			name: "plain-route",
			desc: "Create a new plain route",
			route: networking.NewHTTPRoute(
				"test-route",
				"test-namespace",
				"hostname.example.com",
			),
		},
		{
			name: "route-with-default-gateway",
			desc: "Create a new route with default gateway",
			route: networking.NewHTTPRouteWithDefaultGateway(
				"test-route",
				"test-namespace",
				"hostname.example.com",
			),
		},
		{
			name: "route-with-jupyterhub",
			desc: "Create a new route with jupyterhub",
			route: networking.NewJupyterhubHTTPRoute(
				"test-route",
				"test-namespace",
				"hostname.example.com",
			),
		},
		{
			name: "route-with-airflow",
			desc: "Create a new route with airflow",
			route: networking.NewAirflowHTTPRoute(
				"test-route",
				"test-namespace",
				"hostname.example.com",
			),
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			goldenFilen := goldie.New(t)

			output, err := yaml.Marshal(tc.route)
			if err != nil {
				t.Fatal(err)
			}

			goldenFilen.Assert(t, tc.name, output)
		})
	}
}

func TestHealthCheckPolicy(t *testing.T) {
	testCases := []struct {
		name string
		desc string
		fn   func() (*unstructured.Unstructured, error)
	}{
		{
			name: "plain-healthcheckpolicy",
			desc: "Create a new health check policy",
			fn: func() (*unstructured.Unstructured, error) {
				return networking.NewHealthCheckPolicy(
					"test-policy",
					"test-namespace",
				)
			},
		},
		{
			name: "healthcheckpolicy-with-airflow",
			desc: "Create a new health check policy with airflow",
			fn: func() (*unstructured.Unstructured, error) {
				return networking.NewAirflowHealthCheckPolicy(
					"airflow-test-policy",
					"test-namespace",
				)
			},
		},
		{
			name: "healthcheckpolicy-with-jupyterhub",
			desc: "Create a new health check policy with jupyterhub",
			fn: func() (*unstructured.Unstructured, error) {
				return networking.NewJupyterhubHealthCheckPolicy(
					"jupyter-test-policy",
					"test-namespace",
				)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			goldenFile := goldie.New(t)

			got, err := tc.fn()
			if err != nil {
				t.Fatal(err)
			}

			output, err := yaml.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}

			goldenFile.Assert(t, tc.name, output)
		})
	}
}

func TestNewNetworkPolicy(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		desc   string
		policy *netv1.NetworkPolicy
	}{
		{
			name:   "plain-networkpolicy",
			desc:   "Create a new network policy",
			policy: networking.NewNetworkPolicy("test-policy", "test-namespace", map[string]string{"app": "test-app"}),
		},
		{
			name:   "jupyter-pypi-networkpolicy",
			desc:   "Create a new network policy for jupyterhub that allows access to PyPi",
			policy: networking.NewNetworkPolicyJupyterPyPi("jupyter-policy", "test-namespace"),
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			goldenFile := goldie.New(t)

			output, err := yaml.Marshal(tc.policy)
			if err != nil {
				t.Fatal(err)
			}

			goldenFile.Assert(t, tc.name, output)
		})
	}
}
