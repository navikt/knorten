package networking

import (
	"fmt"
	"github.com/navikt/knorten/pkg/k8s/meta"
	v1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	gwapiv1b1 "sigs.k8s.io/gateway-api/apis/v1beta1"
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

type HTTPRouteOption func(*gwapiv1b1.HTTPRoute)

func WithDefaultGatewayRef() HTTPRouteOption {
	return func(route *gwapiv1b1.HTTPRoute) {
		route.Spec.CommonRouteSpec.ParentRefs = []gwapiv1b1.ParentReference{
			{
				Group:     groupPtr(gwapiv1b1.GroupName),
				Kind:      kindPtr(gatewayKind),
				Namespace: namespacePtr(defaultHTTPRouteSystemNamespace),
				Name:      defaultHTTPRouteName,
			},
		}
	}
}

func WithServiceBackend(serviceName string, port int) HTTPRouteOption {
	return func(route *gwapiv1b1.HTTPRoute) {
		route.Spec.Rules = []gwapiv1b1.HTTPRouteRule{
			{
				BackendRefs: []gwapiv1b1.HTTPBackendRef{
					{
						BackendRef: gwapiv1b1.BackendRef{
							BackendObjectReference: gwapiv1b1.BackendObjectReference{
								Group: groupPtr(groupNameCore),
								Kind:  kindPtr(serviceKind),
								Name:  gwapiv1b1.ObjectName(serviceName),
								Port:  portPtr(port),
							},
						},
					},
				},
			},
		}
	}
}

func NewHTTPRoute(name, namespace, hostname string, options ...HTTPRouteOption) *gwapiv1b1.HTTPRoute {
	route := &gwapiv1b1.HTTPRoute{
		TypeMeta: metav1.TypeMeta{
			Kind:       httpRouteKind,
			APIVersion: gwapiv1b1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    meta.DefaultLabels(),
		},
		Spec: gwapiv1b1.HTTPRouteSpec{
			Hostnames: []gwapiv1b1.Hostname{
				gwapiv1b1.Hostname(hostname),
			},
		},
	}

	for _, option := range options {
		option(route)
	}

	return route
}

func NewHTTPRouteWithDefaultGateway(name, namespace, hostname string, options ...HTTPRouteOption) *gwapiv1b1.HTTPRoute {
	return NewHTTPRoute(name, namespace, hostname, append(options, WithDefaultGatewayRef())...)
}

func NewJupyterhubHTTPRoute(name, namespace, hostname string, options ...HTTPRouteOption) *gwapiv1b1.HTTPRoute {
	options = append(
		options,
		WithServiceBackend(defaultJupyterhubServiceName, defaultJupyterhubPort),
	)

	return NewHTTPRouteWithDefaultGateway(name, namespace, hostname, options...)
}

func NewAirflowHTTPRoute(name, namespace, hostname string, options ...HTTPRouteOption) *gwapiv1b1.HTTPRoute {
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

const (
	fqdnNetpolKind       = "FQDNNetworkPolicy"
	fqdnNetpolAPIVersion = "networking.gke.io/v1alpha3"
)

type FQDNetworkPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec FQDNNetworkPolicySpec `json:"spec,omitempty"`
}

type FQDNNetworkPolicySpec struct {
	PodSelector metav1.LabelSelector          `json:"podSelector,omitempty"`
	Egress      []FQDNNetworkPolicyEgressRule `json:"egress,omitempty"`
	PolicyTypes []netv1.PolicyType            `json:"policyTypes,omitempty"`
}

type FQDNNetworkPolicyEgressRule struct {
	Ports []netv1.NetworkPolicyPort `json:"ports,omitempty"`
	To    []FQDNNetworkPolicyPeer   `json:"to"`
}

type FQDNNetworkPolicyPeer struct {
	FQDNs []string `json:"fqdns"`
}

func NewFQDNNetworkPolicy(name, namespace string, fqdns []string) *FQDNetworkPolicy {
	return &FQDNetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       fqdnNetpolKind,
			APIVersion: fqdnNetpolAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

const (
	networkPolicyKind = "NetworkPolicy"
)

type NetworkPolicyOption func(*netv1.NetworkPolicy)

func WithEgressRule(ports map[int32]string, ipBlocks []string) NetworkPolicyOption {
	return func(policy *netv1.NetworkPolicy) {
		var finalPorts []netv1.NetworkPolicyPort

		for port, protocol := range ports {
			p := v1.Protocol(protocol)

			finalPorts = append(finalPorts, netv1.NetworkPolicyPort{
				Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: port},
				Protocol: &p,
			})
		}

		var finalIPBlocks []netv1.NetworkPolicyPeer

		for _, ipBlock := range ipBlocks {
			finalIPBlocks = append(finalIPBlocks, netv1.NetworkPolicyPeer{
				IPBlock: &netv1.IPBlock{
					CIDR: ipBlock,
				},
			})
		}

		policy.Spec.Egress = append(policy.Spec.Egress, netv1.NetworkPolicyEgressRule{
			Ports: finalPorts,
			To:    finalIPBlocks,
		})

		for _, policyType := range policy.Spec.PolicyTypes {
			if policyType == netv1.PolicyTypeEgress {
				return
			}
		}

		policy.Spec.PolicyTypes = append(policy.Spec.PolicyTypes, netv1.PolicyTypeEgress)
	}
}

func NewNetworkPolicy(name, namespace string, matchLabels map[string]string, options ...NetworkPolicyOption) *netv1.NetworkPolicy {
	p := &netv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       networkPolicyKind,
			APIVersion: netv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    meta.DefaultLabels(),
		},
		Spec: netv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
		},
	}

	for _, option := range options {
		option(p)
	}

	return p
}

func NewNetworkPolicyJupyterPyPi(name, namespace string) *netv1.NetworkPolicy {
	return NewNetworkPolicy(
		name,
		namespace,
		map[string]string{
			"app":       "jupyterhub",
			"component": "singleuser-server",
		},
		// Fastly CDN (for Pypi)
		// curl -v  "https://api.fastly.com/public-ip-list" -H "Accept: application/json"
		WithEgressRule(
			map[int32]string{
				443: "TCP",
			},
			[]string{
				"23.235.32.0/20",
				"43.249.72.0/22",
				"103.244.50.0/24",
				"103.245.222.0/23",
				"103.245.224.0/24",
				"104.156.80.0/20",
				"140.248.64.0/18",
				"140.248.128.0/17",
				"146.75.0.0/17",
				"151.101.0.0/16",
				"157.52.64.0/18",
				"167.82.0.0/17",
				"167.82.128.0/20",
				"167.82.160.0/20",
				"167.82.224.0/20",
				"172.111.64.0/18",
				"185.31.16.0/22",
				"199.27.72.0/21",
				"199.232.0.0/16",
			},
		),
	)
}

func groupPtr(group string) *gwapiv1b1.Group {
	g := gwapiv1b1.Group(group)
	return &g
}

func kindPtr(kind string) *gwapiv1b1.Kind {
	k := gwapiv1b1.Kind(kind)
	return &k
}

func namespacePtr(namespace string) *gwapiv1b1.Namespace {
	n := gwapiv1b1.Namespace(namespace)
	return &n
}

func portPtr(port int) *gwapiv1b1.PortNumber {
	p := gwapiv1b1.PortNumber(port)
	return &p
}
