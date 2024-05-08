package chart

import (
	"context"
	"fmt"

	"github.com/navikt/knorten/pkg/database/gensql"
	"github.com/navikt/knorten/pkg/k8s/core"
	"github.com/navikt/knorten/pkg/k8s/networking"
	v1b1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

const (
	k8sAirflowResourceName    = "airflow-webserver"
	k8sJupyterhubResourceName = "jupyterhub"
	k8sJupyterhubNetworPolicy = "jupyter-notebook-allow-fqdn"
)

func (c Client) deleteSecretFromKubernetes(ctx context.Context, name, namespace string) error {
	return c.manager.DeleteSecret(ctx, name, namespace)
}

func (c Client) createOrUpdateSecret(ctx context.Context, name, namespace string, data map[string]string) error {
	return c.manager.ApplySecret(ctx, core.NewSecret(name, namespace, data))
}

func (c Client) deleteCloudNativePGCluster(ctx context.Context, name, namespace string) error {
	return c.manager.DeletePostgresCluster(ctx, name, namespace)
}

func (c Client) createHttpRoute(ctx context.Context, url, namespace string, chartType gensql.ChartType) error {
	var route *v1b1.HTTPRoute

	switch chartType {
	case gensql.ChartTypeAirflow:
		route = networking.NewAirflowHTTPRoute(k8sAirflowResourceName, namespace, url)
	case gensql.ChartTypeJupyterhub:
		route = networking.NewJupyterhubHTTPRoute(k8sJupyterhubResourceName, namespace, url)
	default:
		return fmt.Errorf("unsupported chart type: %s", chartType)
	}

	return c.manager.ApplyHTTPRoute(ctx, route)
}

func (c Client) deleteHttpRoute(ctx context.Context, namespace string, chartType gensql.ChartType) error {
	var name string

	switch chartType {
	case gensql.ChartTypeAirflow:
		name = k8sAirflowResourceName
	case gensql.ChartTypeJupyterhub:
		name = k8sJupyterhubResourceName
	default:
		return fmt.Errorf("unsupported chart type: %s", chartType)
	}

	return c.manager.DeleteHTTPRoute(ctx, name, namespace)
}

func (c Client) createHealthCheckPolicy(ctx context.Context, namespace string, chartType gensql.ChartType) error {
	switch chartType {
	case gensql.ChartTypeAirflow:
		policy, err := networking.NewAirflowHealthCheckPolicy(k8sAirflowResourceName, namespace)
		if err != nil {
			return err
		}

		return c.manager.ApplyHealthCheckPolicy(ctx, policy)
	case gensql.ChartTypeJupyterhub:
		policy, err := networking.NewJupyterhubHealthCheckPolicy(k8sJupyterhubResourceName, namespace)
		if err != nil {
			return err
		}

		return c.manager.ApplyHealthCheckPolicy(ctx, policy)
	default:
		return fmt.Errorf("unsupported chart type: %s", chartType)
	}
}

func (c Client) deleteHealthCheckPolicy(ctx context.Context, namespace string, chartType gensql.ChartType) error {
	var name string

	switch chartType {
	case gensql.ChartTypeAirflow:
		name = k8sAirflowResourceName
	case gensql.ChartTypeJupyterhub:
		name = k8sJupyterhubResourceName
	}

	return c.manager.DeleteHealthCheckPolicy(ctx, name, namespace)
}

func (c Client) alterJupyterDefaultFQDNNetpol(ctx context.Context, namespace string, enabled bool) error {
	if c.dryRun {
		return nil
	}

	if enabled {
		err := c.manager.ApplyNetworkPolicy(ctx, networking.NewNetworkPolicyJupyterPyPi(k8sJupyterhubNetworPolicy, namespace))
		if err != nil {
			return fmt.Errorf("applying network policy: %w", err)
		}

		return nil
	}

	err := c.manager.DeleteNetworkPolicy(ctx, k8sJupyterhubNetworPolicy, namespace)
	if err != nil {
		return fmt.Errorf("deleting network policy: %w", err)
	}

	return nil
}
