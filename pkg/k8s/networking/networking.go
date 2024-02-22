package networking

import (
	"fmt"
	"github.com/navikt/knorten/pkg/k8s/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
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
	serviceKind                     = "Service"
	groupNameCore                   = "core"
)

type HTTPRouteOption func(*gwapiv1.HTTPRoute)

func WithDefaultGatewayRef() HTTPRouteOption {
	return func(route *gwapiv1.HTTPRoute) {
		route.Spec.CommonRouteSpec.ParentRefs = []gwapiv1.ParentReference{
			{
				Group:     groupPtr(gwapiv1.GroupName),
				Kind:      kindPtr(gatewayKind),
				Namespace: namespacePtr(defaultHTTPRouteSystemNamespace),
				Name:      defaultHTTPRouteName,
			},
		}
	}
}

func WithServiceBackend(serviceName string, port int) HTTPRouteOption {
	return func(route *gwapiv1.HTTPRoute) {
		route.Spec.Rules = []gwapiv1.HTTPRouteRule{
			{
				BackendRefs: []gwapiv1.HTTPBackendRef{
					{
						BackendRef: gwapiv1.BackendRef{
							// Defaults to core API and Service when not defined:
							// - https://gateway-api.sigs.k8s.io/reference/spec/#gateway.networking.k8s.io/v1.BackendObjectReference
							BackendObjectReference: gwapiv1.BackendObjectReference{
								Group: groupPtr(groupNameCore),
								Kind:  kindPtr(serviceKind),
								Name:  gwapiv1.ObjectName(serviceName),
								Port:  portPtr(port),
							},
						},
					},
				},
			},
		}
	}
}

func NewHTTPRoute(name, namespace, hostname string, options ...HTTPRouteOption) *gwapiv1.HTTPRoute {
	route := &gwapiv1.HTTPRoute{
		TypeMeta: metav1.TypeMeta{
			Kind:       httpRouteKind,
			APIVersion: gwapiv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    meta.DefaultLabels(),
		},
		Spec: gwapiv1.HTTPRouteSpec{
			Hostnames: []gwapiv1.Hostname{
				gwapiv1.Hostname(hostname),
			},
		},
	}

	for _, option := range options {
		option(route)
	}

	return route
}

func NewHTTPRouteWithDefaultGateway(name, namespace, hostname string, options ...HTTPRouteOption) *gwapiv1.HTTPRoute {
	return NewHTTPRoute(name, namespace, hostname, append(options, WithDefaultGatewayRef())...)
}

func NewJupyterhubHTTPRoute(name, namespace, hostname string, options ...HTTPRouteOption) *gwapiv1.HTTPRoute {
	options = append(
		options,
		WithServiceBackend(defaultJupyterhubServiceName, defaultJupyterhubPort),
	)

	return NewHTTPRouteWithDefaultGateway(name, namespace, hostname, options...)
}

func NewAirflowHTTPRoute(name, namespace, hostname string, options ...HTTPRouteOption) *gwapiv1.HTTPRoute {
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
	Group string `json:"group"`
	Kind  string `json:"kind,omitempty"`
	Name  string `json:"name,omitempty"`
}

type HealthCheckPolicyOption func(*HealthCheckPolicy)

func WithServiceTargetRef(name string) HealthCheckPolicyOption {
	return func(policy *HealthCheckPolicy) {
		policy.Spec.TargetRef = &HealthCheckPolicySpecTargetRef{
			Group: groupNameCore,
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
			Labels:    meta.DefaultLabels(),
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

func groupPtr(group string) *gwapiv1.Group {
	g := gwapiv1.Group(group)
	return &g
}

func kindPtr(kind string) *gwapiv1.Kind {
	k := gwapiv1.Kind(kind)
	return &k
}

func namespacePtr(namespace string) *gwapiv1.Namespace {
	n := gwapiv1.Namespace(namespace)
	return &n
}

func portPtr(port int) *gwapiv1.PortNumber {
	p := gwapiv1.PortNumber(port)
	return &p
}
