package networking

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	defaultAirflowServiceName       = "airflow-webserver"
	defaultAirflowPort              = 8080
	defaultJupyterhubServiceName    = "proxy-public"
	defaultJupyterhubPort           = 80
	defaultHTTPRouteSystemNamespace = "knada-system"
	defaultHTTPRouteName            = "knada-io"
	httpRouteKind                   = "HTTPRoute"
	gatewayKind                     = "Gateway"
)

type HTTPRouteOption func(*v1.HTTPRoute)

func WithDefaultGatewayRef() HTTPRouteOption {
	return func(route *v1.HTTPRoute) {
		route.Spec.CommonRouteSpec.ParentRefs = []v1.ParentReference{
			{
				Group:     groupPtr(v1.GroupVersion.String()),
				Kind:      kindPtr(gatewayKind),
				Namespace: namespacePtr(defaultHTTPRouteSystemNamespace),
				Name:      defaultHTTPRouteName,
			},
		}
	}
}

func WithServiceBackend(serviceName string, port int) HTTPRouteOption {
	return func(route *v1.HTTPRoute) {
		route.Spec.Rules = []v1.HTTPRouteRule{
			{
				BackendRefs: []v1.HTTPBackendRef{
					{
						BackendRef: v1.BackendRef{
							// Defaults to core API and Service when not defined:
							// - https://gateway-api.sigs.k8s.io/reference/spec/#gateway.networking.k8s.io/v1.BackendObjectReference
							BackendObjectReference: v1.BackendObjectReference{
								Name: v1.ObjectName(serviceName),
								Port: portPtr(port),
							},
						},
					},
				},
			},
		}
	}
}

func NewHTTPRoute(name, namespace, hostname string, options ...HTTPRouteOption) *v1.HTTPRoute {
	route := &v1.HTTPRoute{
		TypeMeta: metav1.TypeMeta{
			Kind:       httpRouteKind,
			APIVersion: v1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1.HTTPRouteSpec{
			Hostnames: []v1.Hostname{
				v1.Hostname(hostname),
			},
		},
	}

	for _, option := range options {
		option(route)
	}

	return route
}

func NewHTTPRouteWithDefaultGateway(name, namespace, hostname string, options ...HTTPRouteOption) *v1.HTTPRoute {
	return NewHTTPRoute(name, namespace, hostname, append(options, WithDefaultGatewayRef())...)
}

func NewJupyterhubHTTPRoute(name, namespace, hostname string, options ...HTTPRouteOption) *v1.HTTPRoute {
	options = append(
		options,
		WithServiceBackend(defaultJupyterhubServiceName, defaultJupyterhubPort),
	)

	return NewHTTPRouteWithDefaultGateway(name, namespace, hostname, options...)
}

func NewAirflowHTTPRoute(name, namespace, hostname string, options ...HTTPRouteOption) *v1.HTTPRoute {
	options = append(
		options,
		WithServiceBackend(defaultAirflowServiceName, defaultAirflowPort),
	)

	return NewHTTPRouteWithDefaultGateway(name, namespace, hostname, options...)
}

const (
	healthCheckPolicyKind       = "HealthCheckPolicy"
	healthCheckPolicyAPIVersion = "networking.gke.io/v1"
	healthCheckPolicyType       = "HTTP"

	serviceKind = "Service"
)

type HealthCheckPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec HealthCheckPolicySpec `json:"spec,omitempty"`
}

type HealthCheckPolicySpec struct {
	Default   *HealthCheckPolicySpecDefault   `json:"default,omitempty"`
	TargetRef *HealthCheckPolicySpecTargetRef `json:"targetRef,omitempty"`
}

type HealthCheckPolicySpecDefault struct {
	Config *HealthCheckPolicySpecDefaultConfig `json:"config,omitempty"`
}

type HealthCheckPolicySpecDefaultConfig struct {
	Type            string                                  `json:"type,omitempty"`
	HTTPHealthCheck *HealthCheckPolicySpecDefaultConfigHTTP `json:"httpHealthCheck,omitempty"`
}

type HealthCheckPolicySpecDefaultConfigHTTP struct {
	RequestPath string `json:"requestPath,omitempty"`
}

type HealthCheckPolicySpecTargetRef struct {
	Group string `json:"group,omitempty"`
	Kind  string `json:"kind,omitempty"`
	Name  string `json:"name,omitempty"`
}

type HealthCheckPolicyOption func(*HealthCheckPolicy)

func WithServiceTargetRef(name string) HealthCheckPolicyOption {
	return func(policy *HealthCheckPolicy) {
		policy.Spec.TargetRef = &HealthCheckPolicySpecTargetRef{
			Group: "",
			Kind:  serviceKind,
			Name:  name,
		}
	}
}

func WithHTTPHealthCheck(requestPath string) HealthCheckPolicyOption {
	return func(policy *HealthCheckPolicy) {
		policy.Spec.Default = &HealthCheckPolicySpecDefault{
			Config: &HealthCheckPolicySpecDefaultConfig{
				Type: healthCheckPolicyType,
				HTTPHealthCheck: &HealthCheckPolicySpecDefaultConfigHTTP{
					RequestPath: requestPath,
				},
			},
		}
	}
}

func NewHealthCheckPolicy(name, namespace string, options ...HealthCheckPolicyOption) (*unstructured.Unstructured, error) {
	policy := &HealthCheckPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       healthCheckPolicyKind,
			APIVersion: healthCheckPolicyAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	for _, option := range options {
		option(policy)
	}

	data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(policy)
	if err != nil {
		return nil, fmt.Errorf("converting health check policy to unstructured: %w", err)
	}

	return &unstructured.Unstructured{
		Object: data,
	}, nil
}

func NewAirflowHealthCheckPolicy(name, namespace string) (*unstructured.Unstructured, error) {
	return NewHealthCheckPolicy(
		name,
		namespace,
		WithServiceTargetRef(defaultAirflowServiceName),
		WithHTTPHealthCheck("/health"),
	)
}

func NewJupyterhubHealthCheckPolicy(name, namespace string) (*unstructured.Unstructured, error) {
	return NewHealthCheckPolicy(
		name,
		namespace,
		WithServiceTargetRef(defaultJupyterhubServiceName),
		WithHTTPHealthCheck("/hub/login"),
	)
}

func groupPtr(group string) *v1.Group {
	g := v1.Group(group)
	return &g
}

func kindPtr(kind string) *v1.Kind {
	k := v1.Kind(kind)
	return &k
}

func namespacePtr(namespace string) *v1.Namespace {
	n := v1.Namespace(namespace)
	return &n
}

func portPtr(port int) *v1.PortNumber {
	p := v1.PortNumber(port)
	return &p
}
