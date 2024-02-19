package chart

import (
	"context"
	"fmt"
	"github.com/navikt/knorten/pkg/database/gensql"
	"github.com/navikt/knorten/pkg/k8s/cnpg"
	"github.com/navikt/knorten/pkg/k8s/core"
	"github.com/navikt/knorten/pkg/k8s/networking"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	cloudSQLProxyName         = "airflow-sql-proxy"
	k8sAirflowResourceName    = "airflow-webserver"
	k8sJupyterhubResourceName = "jupyterhub"
)

var healthCheckPoliciesSchema = schema.GroupVersionResource{
	Group:    "networking.gke.io",
	Version:  "v1",
	Resource: "healthcheckpolicies",
}

var fqdnNetpolSchema = schema.GroupVersionResource{
	Group:    "networking.gke.io",
	Version:  "v1alpha3",
	Resource: "fqdnnetworkpolicies",
}

func (c Client) deleteCloudSQLProxyFromKubernetes(ctx context.Context, namespace string) error {
	if c.dryRun {
		return nil
	}

	if err := c.deleteCloudSQLProxyDeployment(ctx, cloudSQLProxyName, namespace); err != nil {
		return err
	}

	if err := c.deleteCloudSQLProxyService(ctx, cloudSQLProxyName, namespace); err != nil {
		return err
	}

	return nil
}

func (c Client) deleteCloudSQLProxyDeployment(ctx context.Context, name, namespace string) error {
	if err := c.k8sClient.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (c Client) deleteCloudSQLProxyService(ctx context.Context, name, namespace string) error {
	if err := c.k8sClient.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (c Client) deleteSecretFromKubernetes(ctx context.Context, name, namespace string) error {
	return c.manager.DeleteSecret(ctx, name, namespace)
}

func (c Client) createOrUpdateSecret(ctx context.Context, name, namespace string, data map[string]string) error {
	return c.manager.ApplySecret(ctx, core.NewSecret(name, namespace, data))
}

func (c Client) createCloudNativePGCluster(ctx context.Context, name, namespace string) error {
	return c.manager.ApplyPostgresCluster(ctx, cnpg.NewCluster(name, namespace, "airflow", "airflow"))
}

func (c Client) deleteCloudNativePGCluster(ctx context.Context, name, namespace string) error {
	return c.manager.DeletePostgresCluster(ctx, name, namespace)
}

func (c Client) createHttpRoute(ctx context.Context, url, namespace string, chartType gensql.ChartType) error {
	var route *v1.HTTPRoute

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

func (c Client) createHealtCheckPolicy(ctx context.Context, namespace string, chartType gensql.ChartType) error {
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

func (c Client) deleteHealtCheckPolicy(ctx context.Context, namespace string, chartType gensql.ChartType) error {
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
		return c.createJupyterDefaultFQDNNetpol(ctx, namespace)
	}

	return c.deleteJupyterDefaultFQDNNetpol(ctx, namespace)
}

func (c Client) createJupyterDefaultFQDNNetpol(ctx context.Context, namespace string) error {
	fqdnNetpol := createFQDNNetpol(namespace)

	existing, err := c.k8sDynamicClient.Resource(fqdnNetpolSchema).Namespace(namespace).Get(ctx, fqdnNetpol.GetName(), metav1.GetOptions{})
	if err == nil {
		existing.Object["spec"] = fqdnNetpol.Object["spec"]
		_, err := c.k8sDynamicClient.Resource(fqdnNetpolSchema).Namespace(namespace).Update(ctx, existing, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	} else if k8sErrors.IsNotFound(err) {
		_, err = c.k8sDynamicClient.Resource(fqdnNetpolSchema).Namespace(namespace).Create(ctx, fqdnNetpol, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	} else {
		return err
	}

	return nil
}

func (c Client) deleteJupyterDefaultFQDNNetpol(ctx context.Context, namespace string) error {
	err := c.k8sDynamicClient.Resource(fqdnNetpolSchema).Namespace(namespace).Delete(ctx, "jupyter-notebook-allow-fqdn", metav1.DeleteOptions{})
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}

	return nil
}

func createFQDNNetpol(namespace string) *unstructured.Unstructured {
	fqdnNetpol := &unstructured.Unstructured{}
	fqdnNetpol.SetUnstructuredContent(map[string]interface{}{
		"apiVersion": "networking.gke.io/v1alpha3",
		"kind":       "FQDNNetworkPolicy",
		"metadata": map[string]any{
			"name":      "jupyter-notebook-allow-fqdn",
			"namespace": namespace,
			"labels": map[string]string{
				"app.kubernetes.io/managed-by": "knorten",
			},
		},
		"spec": map[string]any{
			"podSelector": map[string]any{
				"matchLabels": map[string]string{
					"app":       "jupyterhub",
					"component": "singleuser-server",
				},
			},
			"egress": []map[string]any{
				{
					"ports": []map[string]any{
						{
							"port":     443,
							"protocol": "TCP",
						},
					},
					"to": []map[string]any{
						{
							"fqdns": []string{
								"pypi.org",
								"files.pythonhosted.org",
								"pypi.python.org",
							},
						},
					},
				},
			},
			"policyTypes": []string{
				"Egress",
			},
		},
	})

	return fqdnNetpol
}
