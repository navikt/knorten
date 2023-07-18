package team

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c Client) createK8sNamespace(ctx context.Context, name string) error {
	if c.dryRun {
		c.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"team-namespace":           "true",
				"allow-all-jupyter-egress": "true",
			},
		},
	}

	_, err := c.k8sClient.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		if k8sErrors.IsAlreadyExists(err) {
			c.log.Infof("namespace %v already exists", name)
			return nil
		}
		c.log.WithError(err).Error("creating team namespace")
		return err
	}

	return nil
}

func (c Client) deleteK8sNamespace(ctx context.Context, namespace string) error {
	if c.dryRun {
		c.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	err := c.k8sClient.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			c.log.Infof("delete namespace: namespace %v does not exist", namespace)
			return nil
		}
		c.log.WithError(err).Error("deleting team namespace")
		return err
	}
	return nil
}

func (c Client) createK8sServiceAccount(ctx context.Context, teamID, namespace string) error {
	if c.dryRun {
		c.log.Infof("NOOP: Running in dry run mode")
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
	if err != nil {
		if k8sErrors.IsAlreadyExists(err) {
			c.log.Infof("service account %v already exists in namespace %v", teamID, namespace)
			return nil
		}
		c.log.WithError(err).Error("creating team service account")
		return err
	}

	return nil
}
