package chart

import (
	"context"
	"fmt"

	"github.com/navikt/knorten/pkg/database/gensql"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/gateway-api/apis/v1beta1"
)

const (
	cloudSQLProxyName                 = "airflow-sql-proxy"
	k8sLabelEnableTeamNetworkPolicies = "team-netpols"
	k8sAirflowResourceName            = "airflow-webserver"
	k8sJupyterhubResourceName         = "jupyterhub"
)

var healthCheckPoliciesSchema = schema.GroupVersionResource{
	Group:    "networking.gke.io",
	Version:  "v1",
	Resource: "healthcheckpolicies",
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
	if c.dryRun {
		return nil
	}

	if err := c.k8sClient.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (c Client) createOrUpdateSecret(ctx context.Context, name, namespace string, data map[string]string) error {
	if c.dryRun {
		return nil
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		StringData: data,
	}

	_, err := c.k8sClient.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if !k8sErrors.IsNotFound(err) {
			return err
		}

		_, err = c.k8sClient.CoreV1().Secrets(secret.Namespace).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			return err
		}

		return nil
	}

	_, err = c.k8sClient.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (c Client) createOrUpdateCloudSQLProxy(ctx context.Context, name, teamID, namespace, dbInstance string) error {
	if c.dryRun {
		return nil
	}

	port := int32(5432)

	if err := c.createOrUpdateCloudSQLProxyDeployment(ctx, name, namespace, teamID, dbInstance, port); err != nil {
		return err
	}

	if err := c.createOrUpdateCloudSQLProxyService(ctx, name, namespace, port); err != nil {
		return err
	}

	return nil
}

func (c Client) createOrUpdateCloudSQLProxyDeployment(ctx context.Context, name, namespace, saName, dbInstance string, port int32) error {
	runAsNonRoot := true
	allowPrivilegeEscalation := false
	userID := int64(65532)
	groupID := int64(65532)

	deploySpec := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "cloudsql-proxy",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "cloudsql-proxy",
					},
				},
				Spec: v1.PodSpec{
					ServiceAccountName: saName,
					Containers: []v1.Container{
						{
							Name:  "cloudsql-proxy",
							Image: "gcr.io/cloud-sql-connectors/cloud-sql-proxy:2.0.0-alpine",
							Ports: []v1.ContainerPort{
								{
									Protocol:      v1.ProtocolTCP,
									ContainerPort: port,
								},
							},
							Command: []string{
								"/cloud-sql-proxy",
								"--max-sigterm-delay=30s",
								"--address=0.0.0.0",
								fmt.Sprintf("--port=%v", port),
								fmt.Sprintf("%v:%v:%v", c.gcpProject, c.gcpRegion, dbInstance),
							},
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("20m"),
									v1.ResourceMemory: resource.MustParse("32Mi"),
								},
								Limits: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("100m"),
									v1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
							SecurityContext: &v1.SecurityContext{
								RunAsUser:                &userID,
								RunAsGroup:               &groupID,
								AllowPrivilegeEscalation: &allowPrivilegeEscalation,
							},
						},
					},
					SecurityContext: &v1.PodSecurityContext{
						RunAsNonRoot: &runAsNonRoot,
					},
				},
			},
		},
	}

	_, err := c.k8sClient.AppsV1().Deployments(namespace).Create(ctx, deploySpec, metav1.CreateOptions{})
	if err != nil {
		if !k8sErrors.IsAlreadyExists(err) {
			return err
		}
		_, err := c.k8sClient.AppsV1().Deployments(namespace).Update(ctx, deploySpec, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (c Client) createOrUpdateCloudSQLProxyService(ctx context.Context, name, namespace string, port int32) error {
	serviceSpec := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"app": "cloudsql-proxy",
			},
			Ports: []v1.ServicePort{
				{
					Protocol:   v1.ProtocolTCP,
					Port:       port,
					TargetPort: intstr.IntOrString{IntVal: port},
				},
			},
		},
	}

	_, err := c.k8sClient.CoreV1().Services(namespace).Create(ctx, serviceSpec, metav1.CreateOptions{})
	if err != nil {
		if !k8sErrors.IsAlreadyExists(err) {
			return err
		}
		_, err := c.k8sClient.CoreV1().Services(namespace).Update(ctx, serviceSpec, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (c Client) createHttpRoute(ctx context.Context, url, namespace string, chartType gensql.ChartType) error {
	if c.dryRun {
		return nil
	}

	gatewayGroup := v1beta1.Group("gateway.networking.k8s.io")
	gatewayKind := v1beta1.Kind("Gateway")
	gatewayNamespace := v1beta1.Namespace("knada-system")
	httpRoute := &v1beta1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
		},
		Spec: v1beta1.HTTPRouteSpec{
			CommonRouteSpec: v1beta1.CommonRouteSpec{
				ParentRefs: []v1beta1.ParentReference{
					{
						Kind:      &gatewayKind,
						Group:     &gatewayGroup,
						Namespace: &gatewayNamespace,
						Name:      "knada-io",
					},
				},
			},
			Hostnames: []v1beta1.Hostname{v1beta1.Hostname(url)},
			Rules: []v1beta1.HTTPRouteRule{
				{
					BackendRefs: []v1beta1.HTTPBackendRef{
						{
							BackendRef: v1beta1.BackendRef{},
						},
					},
				},
			},
		},
	}

	switch chartType {
	case gensql.ChartTypeAirflow:
		httpRoute.Name = k8sAirflowResourceName
		portNumber := v1beta1.PortNumber(8080)
		httpRoute.Spec.Rules[0].BackendRefs[0].BackendRef = v1beta1.BackendRef{
			BackendObjectReference: v1beta1.BackendObjectReference{
				Name: "airflow-webserver",
				Port: &portNumber,
			},
		}
	case gensql.ChartTypeJupyterhub:
		httpRoute.Name = k8sJupyterhubResourceName
		portNumber := v1beta1.PortNumber(80)
		httpRoute.Spec.Rules[0].BackendRefs[0].BackendRef = v1beta1.BackendRef{
			BackendObjectReference: v1beta1.BackendObjectReference{
				Name: "proxy-public",
				Port: &portNumber,
			},
		}
	}

	_, err := c.k8sGatewayClient.GatewayV1beta1().HTTPRoutes(namespace).Create(ctx, httpRoute, metav1.CreateOptions{})
	if err != nil && !k8sErrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func (c Client) deleteHttpRoute(ctx context.Context, namespace string, chartType gensql.ChartType) error {
	if c.dryRun {
		return nil
	}

	name := ""
	switch chartType {
	case gensql.ChartTypeAirflow:
		name = k8sAirflowResourceName
	case gensql.ChartTypeJupyterhub:
		name = k8sJupyterhubResourceName
	}

	err := c.k8sGatewayClient.GatewayV1beta1().HTTPRoutes(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (c Client) createHealtCheckPolicy(ctx context.Context, namespace string, chartType gensql.ChartType) error {
	if c.dryRun {
		return nil
	}

	name := ""
	serviceName := ""
	requestPath := ""
	switch chartType {
	case gensql.ChartTypeAirflow:
		name = k8sAirflowResourceName
		serviceName = "airflow-webserver"
		requestPath = "/health"
	case gensql.ChartTypeJupyterhub:
		name = k8sJupyterhubResourceName
		serviceName = "proxy-public"
		requestPath = "/hub/login"
	}

	healthCheckPolicy := &unstructured.Unstructured{}
	healthCheckPolicy.SetUnstructuredContent(map[string]interface{}{
		"apiVersion": "networking.gke.io/v1",
		"kind":       "HealthCheckPolicy",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"default": map[string]interface{}{
				"config": map[string]interface{}{
					"type": "HTTP",
					"httpHealthCheck": map[string]interface{}{
						"requestPath": requestPath,
					},
				},
			},
			"targetRef": map[string]interface{}{
				"group": "",
				"kind":  "Service",
				"name":  serviceName,
			},
		},
	})

	_, err := c.k8sDynamicClient.Resource(healthCheckPoliciesSchema).Namespace(namespace).Create(ctx, healthCheckPolicy, metav1.CreateOptions{})
	if err != nil && !k8sErrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func (c Client) deleteHealtCheckPolicy(ctx context.Context, namespace string, chartType gensql.ChartType) error {
	if c.dryRun {
		return nil
	}

	name := ""
	switch chartType {
	case gensql.ChartTypeAirflow:
		name = k8sAirflowResourceName
	case gensql.ChartTypeJupyterhub:
		name = k8sJupyterhubResourceName
	}
	err := c.k8sDynamicClient.Resource(healthCheckPoliciesSchema).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}

	return nil
}
