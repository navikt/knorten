package networking_test

import (
	"github.com/navikt/knorten/pkg/k8s/networking"
	"github.com/sebdah/goldie/v2"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	"sigs.k8s.io/yaml"
	"testing"
)

func TestHTTPRoute(t *testing.T) {
	testCases := []struct {
		name  string
		desc  string
		route *v1.HTTPRoute
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

			g := goldie.New(t)

			d, err := yaml.Marshal(tc.route)
			if err != nil {
				t.Fatal(err)
			}

			g.Assert(t, tc.name, d)
		})
	}
}
