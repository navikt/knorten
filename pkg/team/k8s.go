package team

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	k8sLabelEnableTeamNetworkPolicies = "team-netpols"
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

func (c Client) defaultEgressNetpolSync(ctx context.Context, namespace string, restrictEgress bool) error {
	if c.dryRun {
		return nil
	}

	nsSpec, err := c.k8sClient.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if restrictEgress {
		nsSpec.Labels[k8sLabelEnableTeamNetworkPolicies] = "true"
	} else {
		delete(nsSpec.Labels, k8sLabelEnableTeamNetworkPolicies)
		err := c.k8sClient.NetworkingV1().NetworkPolicies(namespace).Delete(ctx, k8sLabelEnableTeamNetworkPolicies, metav1.DeleteOptions{})
		if err != nil && !k8sErrors.IsNotFound(err) {
			return err
		}
	}

	_, err = c.k8sClient.CoreV1().Namespaces().Update(ctx, nsSpec, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}
