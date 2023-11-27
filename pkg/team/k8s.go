package team

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	k8sLabelEnableTeamNetworkPolicies = "team-netpols"
	replicatorLabel                   = "app.kubernetes.io/managed-by=replicator"
)

func (c Client) k8sNamespaceExists(ctx context.Context, namespace string) (bool, error) {
	if c.dryRun {
		return false, nil
	}

	_, err := c.k8sClient.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (c Client) createK8sNamespace(ctx context.Context, name string) error {
	if c.dryRun {
		return nil
	}

	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"team-namespace": "true",
			},
		},
	}

	_, err := c.k8sClient.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil && !k8sErrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func (c Client) deleteK8sNamespace(ctx context.Context, namespace string) error {
	if c.dryRun {
		return nil
	}

	err := c.k8sClient.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
	if err != nil && !k8sErrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (c Client) k8sServiceAccountExists(ctx context.Context, teamID, namespace string) (bool, error) {
	if c.dryRun {
		return false, nil
	}

	_, err := c.k8sClient.CoreV1().ServiceAccounts(namespace).Get(ctx, teamID, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (c Client) createK8sServiceAccount(ctx context.Context, teamID, namespace string) error {
	if c.dryRun {
		return nil
	}

	saSpec := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      teamID,
			Namespace: namespace,
			Annotations: map[string]string{
				"iam.gke.io/gcp-service-account": fmt.Sprintf("%v@%v.iam.gserviceaccount.com", teamID, c.gcpProject),
			},
		},
	}

	_, err := c.k8sClient.CoreV1().ServiceAccounts(namespace).Create(ctx, saSpec, metav1.CreateOptions{})
	if err != nil && !k8sErrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func (c Client) removeReplicatorNetpols(ctx context.Context, namespace string) error {
	if err := c.removeFQDNNetpols(ctx, namespace); err != nil {
		return err
	}

	return c.removeRegularNetpols(ctx, namespace)
}

func (c Client) removeFQDNNetpols(ctx context.Context, namespace string) error {
	fqdnNetpolResource := schema.GroupVersionResource{
		Group:    "networking.gke.io",
		Version:  "v1alpha3",
		Resource: "fqdnnetworkpolicies",
	}
	fqdnNetpols, err := c.k8sDynamicClient.Resource(fqdnNetpolResource).Namespace(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: replicatorLabel,
	})
	if err != nil {
		return err
	}

	for _, fqdn := range fqdnNetpols.Items {
		if err := c.k8sDynamicClient.Resource(fqdnNetpolResource).Namespace(namespace).Delete(ctx, fqdn.GetName(), metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (c Client) removeRegularNetpols(ctx context.Context, namespace string) error {
	netpols, err := c.k8sClient.NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: replicatorLabel,
	})
	if err != nil {
		return err
	}

	for _, netpol := range netpols.Items {
		err := c.k8sClient.NetworkingV1().NetworkPolicies(namespace).Delete(ctx, netpol.Name, metav1.DeleteOptions{})
		if err != nil && !k8sErrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}
