package k8s

import (
	"context"
	"fmt"
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	"github.com/navikt/knorten/pkg/k8s/networking"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	"strings"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

const (
	fieldManager = "knorten"
)

func NewClient(context string) (client.Client, error) {
	cfg, err := config.GetConfigWithContext(context)
	if err != nil {
		return nil, fmt.Errorf("getting kubeconfig: %w", err)
	}

	c, err := client.New(cfg, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("creating k8s client: %w", err)
	}

	scheme := c.Scheme()
	if err := cnpgv1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("adding cloudnative-pg scheme: %w", err)
	}

	if err := gwapiv1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("adding gateway-api scheme: %w", err)
	}

	return c, nil
}

// NewDryRunClient creates a dry run client which will not apply any
// actual changes to the cluster.
func NewDryRunClient(c client.Client) client.Client {
	return client.NewDryRunClient(c)
}

type Manager interface {
	ApplyPostgresCluster(ctx context.Context, cluster *cnpgv1.Cluster) error
	DeletePostgresCluster(ctx context.Context, name, namespace string) error
	ApplySecret(ctx context.Context, secret *v1.Secret) error
	DeleteSecret(ctx context.Context, name, namespace string) error
	ApplyHTTPRoute(ctx context.Context, route *gwapiv1.HTTPRoute) error
	DeleteHTTPRoute(ctx context.Context, name, namespace string) error
	ApplyHealthCheckPolicy(ctx context.Context, policy *unstructured.Unstructured) error
	DeleteHealthCheckPolicy(ctx context.Context, name, namespace string) error
	ApplyNamespace(ctx context.Context, namespace *v1.Namespace) error
	DeleteNamespace(ctx context.Context, name string) error
	ApplyServiceAccount(ctx context.Context, serviceAccount *v1.ServiceAccount) error
	DeleteServiceAccount(ctx context.Context, name, namespace string) error
}

type manager struct {
	client client.Client
}

func (m *manager) ApplyServiceAccount(ctx context.Context, serviceAccount *v1.ServiceAccount) error {
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

func (m *manager) ApplyHTTPRoute(ctx context.Context, route *gwapiv1.HTTPRoute) error {
	err := m.apply(ctx, route)
	if err != nil {
		return fmt.Errorf("applying httproute: %w", err)
	}

	return nil
}

func (m *manager) DeleteHTTPRoute(ctx context.Context, name, namespace string) error {
	err := m.delete(ctx, &gwapiv1.HTTPRoute{
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

func (m *manager) ApplyHealthCheckPolicy(ctx context.Context, policy *unstructured.Unstructured) error {
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

func (m *manager) apply(ctx context.Context, obj client.Object) error {
	existing, ok := obj.DeepCopyObject().(client.Object)
	if !ok {
		return fmt.Errorf("unable to cast object to client.Object")
	}

	err := m.client.Get(ctx, client.ObjectKeyFromObject(obj), existing)
	if err != nil {
		// If the resource does not exist, we create it
		if errors.IsNotFound(err) {
			return m.client.Create(ctx, obj)
		}

		return fmt.Errorf("checking resource: %w", err)
	}

	// Otherwise, we update it
	return m.client.Patch(ctx, obj, client.Apply, &client.PatchOptions{
		FieldManager: fieldManager,
	})
}

func NewManager(client client.Client) Manager {
	return &manager{
		client: client,
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
