package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	"github.com/navikt/knorten/pkg/k8s/core"
	"github.com/navikt/knorten/pkg/k8s/networking"
	v1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	gwapiv1b1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

const (
	fieldManager = "knorten"
)

type Client struct {
	client.Client
	RESTConfig *rest.Config
	KubeConfig *KubeConfig
}

type (
	SchemeAdderFn func(scheme *runtime.Scheme) error
	IsReadyFn     func(*unstructured.Unstructured) bool
)

func DefaultSchemeAdder() SchemeAdderFn {
	return func(scheme *runtime.Scheme) error {
		if err := cnpgv1.AddToScheme(scheme); err != nil {
			return fmt.Errorf("adding cloudnative-pg scheme: %w", err)
		}

		if err := gwapiv1b1.Install(scheme); err != nil {
			return fmt.Errorf("adding gateway-api scheme: %w", err)
		}

		return nil
	}
}

func NewClient(context string, fn SchemeAdderFn) (*Client, error) {
	cfg, err := config.GetConfigWithContext(context)
	if err != nil {
		return nil, fmt.Errorf("getting kubeconfig: %w", err)
	}

	cfg.Timeout = 5 * time.Second

	c, err := client.New(cfg, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("creating k8s client: %w", err)
	}

	if fn != nil {
		scheme := c.Scheme()
		if err := fn(scheme); err != nil {
			return nil, fmt.Errorf("adding scheme: %w", err)
		}
	}

	kubeConfig := NewKubeConfig("knorten")

	err = kubeConfig.FromREST(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating kubeconfig: %w", err)
	}

	log.SetLogger(klog.NewKlogr())

	return &Client{
		Client:     c,
		RESTConfig: cfg,
		KubeConfig: kubeConfig,
	}, nil
}

// NewDryRunClient creates a dry run client which will not apply any
// actual changes to the cluster.
func NewDryRunClient(c client.Client) client.Client {
	return client.NewDryRunClient(c)
}

type Manager interface {
	ApplyPostgresCluster(ctx context.Context, cluster *cnpgv1.Cluster) error
	DeletePostgresCluster(ctx context.Context, name, namespace string) error
	ApplyScheduledBackup(ctx context.Context, backup *cnpgv1.ScheduledBackup) error
	DeleteScheduledBackup(ctx context.Context, name, namespace string) error
	ApplySecret(ctx context.Context, secret *v1.Secret) error
	DeleteSecret(ctx context.Context, name, namespace string) error
	GetSecret(ctx context.Context, name, namespace string) (*v1.Secret, error)
	WaitForSecret(ctx context.Context, name, namespace string) (*v1.Secret, error)
	ApplyHTTPRoute(ctx context.Context, route *gwapiv1b1.HTTPRoute) error
	DeleteHTTPRoute(ctx context.Context, name, namespace string) error
	ApplyHealthCheckPolicy(ctx context.Context, policy *unstructured.Unstructured) error
	DeleteHealthCheckPolicy(ctx context.Context, name, namespace string) error
	ApplyNamespace(ctx context.Context, namespace *v1.Namespace) error
	DeleteNamespace(ctx context.Context, name string) error
	ApplyServiceAccount(ctx context.Context, serviceAccount *v1.ServiceAccount) error
	DeleteServiceAccount(ctx context.Context, name, namespace string) error
	ApplyNetworkPolicy(ctx context.Context, policy *netv1.NetworkPolicy) error
	DeleteNetworkPolicy(ctx context.Context, name, namespace string) error
	DeletePodsWithLabels(ctx context.Context, namespace, lables string) error
	GetStatusForPodsWithLabels(ctx context.Context, namespace, labels string) ([]v1.PodStatus, error)
}

type manager struct {
	client *Client
}

func (m *manager) ApplyScheduledBackup(ctx context.Context, backup *cnpgv1.ScheduledBackup) error {
	err := m.apply(ctx, backup)
	if err != nil {
		return fmt.Errorf("applying scheduled backup: %w", err)
	}

	return nil
}

func (m *manager) DeleteScheduledBackup(ctx context.Context, name, namespace string) error {
	err := m.delete(ctx, &cnpgv1.ScheduledBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	})
	if err != nil {
		return fmt.Errorf("deleting scheduled backup: %w", err)
	}

	return nil
}

func (m *manager) ApplyNetworkPolicy(ctx context.Context, policy *netv1.NetworkPolicy) error {
	err := m.apply(ctx, policy)
	if err != nil {
		return fmt.Errorf("applying networkpolicy: %w", err)
	}

	return nil
}

func (m *manager) DeleteNetworkPolicy(ctx context.Context, name, namespace string) error {
	err := m.delete(ctx, &netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	})
	if err != nil {
		return fmt.Errorf("deleting networkpolicy: %w", err)
	}

	return nil
}

func (m *manager) GetSecret(ctx context.Context, name, namespace string) (*v1.Secret, error) {
	secret, err := m.get(ctx, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("getting secret: %w", err)
	}

	s, ok := secret.(*v1.Secret)
	if !ok {
		return nil, fmt.Errorf("unable to cast object to secret")
	}

	return s, nil
}

func (m *manager) ApplyServiceAccount(
	ctx context.Context,
	serviceAccount *v1.ServiceAccount,
) error {
	err := m.apply(ctx, serviceAccount)
	if err != nil {
		return fmt.Errorf("applying serviceaccount: %w", err)
	}

	return nil
}

func (m *manager) DeleteServiceAccount(ctx context.Context, name, namespace string) error {
	err := m.delete(ctx, &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	})
	if err != nil {
		return fmt.Errorf("deleting serviceaccount: %w", err)
	}

	return nil
}

func (m *manager) ApplyNamespace(ctx context.Context, namespace *v1.Namespace) error {
	err := m.apply(ctx, namespace)
	if err != nil {
		return fmt.Errorf("applying namespace: %w", err)
	}

	return nil
}

func (m *manager) DeleteNamespace(ctx context.Context, name string) error {
	err := m.delete(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	})
	if err != nil {
		return fmt.Errorf("deleting namespace: %w", err)
	}

	return nil
}

func (m *manager) ApplyPostgresCluster(ctx context.Context, cluster *cnpgv1.Cluster) error {
	err := m.apply(ctx, cluster)
	if err != nil {
		return fmt.Errorf("applying postgres cluster: %w", err)
	}

	return nil
}

func (m *manager) DeletePostgresCluster(ctx context.Context, name, namespace string) error {
	err := m.delete(ctx, &cnpgv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	})
	if err != nil {
		return fmt.Errorf("deleting postgres cluster: %w", err)
	}

	return nil
}

func (m *manager) ApplySecret(ctx context.Context, secret *v1.Secret) error {
	err := m.apply(ctx, secret)
	if err != nil {
		return fmt.Errorf("applying secret: %w", err)
	}

	return nil
}

func (m *manager) DeleteSecret(ctx context.Context, name, namespace string) error {
	err := m.delete(ctx, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	})
	if err != nil {
		return fmt.Errorf("deleting core: %w", err)
	}

	return nil
}

func SecretIsReadyFn() IsReadyFn {
	// A Secret is ready when it exists
	return func(obj *unstructured.Unstructured) bool {
		return true
	}
}

func (m *manager) WaitForSecret(ctx context.Context, name, namespace string) (*v1.Secret, error) {
	var cancelFn context.CancelFunc

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		ctx, cancelFn = context.WithDeadline(ctx, time.Now().Add(2*time.Minute))
	}

	defer func() {
		if cancelFn != nil {
			cancelFn()
		}
	}()

	into := &v1.Secret{}

	err := m.waitForResource(ctx, core.NewSecret(name, namespace, nil), into, SecretIsReadyFn())
	if err != nil {
		return nil, fmt.Errorf("waiting for secret: %w", err)
	}

	return into, nil
}

func (m *manager) ApplyHTTPRoute(ctx context.Context, route *gwapiv1b1.HTTPRoute) error {
	err := m.apply(ctx, route)
	if err != nil {
		return fmt.Errorf("applying httproute: %w", err)
	}

	return nil
}

func (m *manager) DeleteHTTPRoute(ctx context.Context, name, namespace string) error {
	err := m.delete(ctx, &gwapiv1b1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	})
	if err != nil {
		return fmt.Errorf("deleting httproute: %w", err)
	}

	return nil
}

func (m *manager) ApplyHealthCheckPolicy(
	ctx context.Context,
	policy *unstructured.Unstructured,
) error {
	err := m.apply(ctx, policy)
	if err != nil {
		return fmt.Errorf("applying healthcheckpolicy: %w", err)
	}

	return nil
}

func (m *manager) DeleteHealthCheckPolicy(ctx context.Context, name, namespace string) error {
	policy, err := networking.NewHealthCheckPolicy(name, namespace)
	if err != nil {
		return fmt.Errorf("creating healthcheckpolicy: %w", err)
	}

	err = m.delete(ctx, policy)
	if err != nil {
		return fmt.Errorf("deleting healthcheckpolicy: %w", err)
	}

	return nil
}

func (m *manager) DeletePodsWithLabels(ctx context.Context, namespace, lables string) error {
	podlist := &v1.PodList{}

	err := m.list(ctx, namespace, lables, podlist)
	if err != nil {
		return fmt.Errorf("listing pods with lables: %w", err)
	}

	for _, pod := range podlist.Items {
		err = m.delete(ctx, &pod)
		if err != nil {
			return fmt.Errorf("deleting pod: %w", err)
		}
	}

	return nil
}

func (m *manager) GetStatusForPodsWithLabels(
	ctx context.Context,
	namespace, labels string,
) ([]v1.PodStatus, error) {
	podlist := &v1.PodList{}

	statuses := []v1.PodStatus{}

	err := m.list(ctx, namespace, labels, podlist)
	if err != nil {
		return statuses, fmt.Errorf("listing pods with lables: %w", err)
	}

	for _, pod := range podlist.Items {
		statuses = append(statuses, pod.Status)
	}

	return statuses, nil
}

func (m *manager) waitForResource(
	ctx context.Context,
	from client.Object,
	toPtr any,
	fn IsReadyFn,
) error {
	watcher, err := client.NewWithWatch(m.client.RESTConfig, client.Options{})
	if err != nil {
		return fmt.Errorf("creating client with watcher: %w", err)
	}

	name := from.GetName()
	namespace := from.GetNamespace()

	resource, err := runtime.DefaultUnstructuredConverter.ToUnstructured(from)
	if err != nil {
		return fmt.Errorf("converting to unstructured: %w", err)
	}

	w, err := watcher.Watch(ctx, &unstructured.Unstructured{
		Object: resource,
	}, client.InNamespace(namespace))
	if err != nil {
		return fmt.Errorf("creating resource watcher: %w", err)
	}

	defer w.Stop()

	ctxDoneCh := ctx.Done()

	// FIXME: Set a default timeout
	var timeoutCh <-chan time.Time
	if deadline, ok := ctx.Deadline(); ok {
		timeoutCh = time.After(time.Until(deadline))
	}

	for {
		select {
		case <-timeoutCh:
			return fmt.Errorf("timed out waiting for resource: %s/%s", namespace, name)
		case <-ctxDoneCh:
			return fmt.Errorf("context done when waiting for resource: %s/%s", namespace, name)
		case evt := <-w.ResultChan():
			if evt.Type == watch.Error {
				return fmt.Errorf("watcher error: %v", evt.Object)
			}

			obj, ok := evt.Object.(*unstructured.Unstructured)
			if !ok {
				return fmt.Errorf("unable to cast object to unstructured")
			}

			if obj.GetName() == from.GetName() && fn(obj) {
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, toPtr)
				if err != nil {
					return fmt.Errorf("converting unstructured: %w", err)
				}

				return nil
			}
		default:
			time.Sleep(5 * time.Second)
		}
	}
}

func (m *manager) delete(ctx context.Context, obj client.Object) error {
	existing, ok := obj.DeepCopyObject().(client.Object)
	if !ok {
		return fmt.Errorf("unable to cast object to client.Object")
	}

	err := m.client.Get(ctx, client.ObjectKeyFromObject(obj), existing)
	if err != nil {
		// If the resource does not exist, we consider it deleted
		if errors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("checking resource: %w", err)
	}

	return m.client.Delete(ctx, existing)
}

func (m *manager) get(ctx context.Context, into client.Object) (client.Object, error) {
	err := m.client.Get(ctx, client.ObjectKeyFromObject(into), into)
	if err != nil {
		return nil, fmt.Errorf("getting resource: %w", err)
	}

	return into, nil
}

func (m *manager) apply(ctx context.Context, obj client.Object) error {
	existing, ok := obj.DeepCopyObject().(client.Object)
	if !ok {
		return fmt.Errorf("unable to cast object to client.Object")
	}

	err := m.client.Get(ctx, client.ObjectKeyFromObject(obj), existing)
	if err != nil {
		// If the resource does not exist, we create it
		if errors.IsNotFound(err) {
			err = m.client.Create(ctx, obj, &client.CreateOptions{
				FieldManager: fieldManager,
			})
			if err != nil {
				return fmt.Errorf("creating resource: %w", err)
			}

			return nil
		}

		return fmt.Errorf("checking resource: %w", err)
	}

	// Otherwise, we update it
	err = m.client.Patch(ctx, obj, client.Apply, &client.PatchOptions{
		Force:        ptr.To(true), // Need to force the update to take ownership of the resource
		FieldManager: fieldManager,
	})
	if err != nil {
		return fmt.Errorf("patching resource: %w", err)
	}

	return nil
}

func (m *manager) list(
	ctx context.Context,
	namespace string,
	labelSelectorString string,
	obj client.ObjectList,
) error {
	labelSelector, err := labels.Parse(labelSelectorString)
	if err != nil {
		return fmt.Errorf("parsing label selector: %w", err)
	}

	err = m.client.List(ctx, obj, &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("listing resources: %w", err)
	}

	return nil
}

func NewManager(c *Client) Manager {
	return &manager{
		client: c,
	}
}

// TeamIDToNamespace prefix team- to a team ID. If the ID already has in as a prefix, will add a - after the word team.
//
// hello-1234 => team-hello-1234
//
// teamhello-1234 => team-hello-1234
//
// helloteam-1234 => team-helloteam-1234
func TeamIDToNamespace(name string) string {
	if strings.HasPrefix(name, "team-") {
		return name
	} else if strings.HasPrefix(name, "team") {
		return strings.Replace(name, "team", "team-", 1)
	} else {
		return fmt.Sprintf("team-%v", name)
	}
}
