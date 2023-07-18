package chart

import (
	"context"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	cloudSQLProxyName = "airflow-sql-proxy"
)

func (a airflowClient) deleteCloudSQLProxy(ctx context.Context, namespace string) error {
	if a.dryRun {
		a.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	if err := a.deleteCloudSQLProxyDeployment(ctx, cloudSQLProxyName, namespace); err != nil {
		return err
	}

	if err := a.deleteCloudSQLProxyService(ctx, cloudSQLProxyName, namespace); err != nil {
		return err
	}

	return nil
}

func (a airflowClient) deleteCloudSQLProxyDeployment(ctx context.Context, name, namespace string) error {
	if err := a.k8sClient.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if !k8sErrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func (a airflowClient) deleteCloudSQLProxyService(ctx context.Context, name, namespace string) error {
	if err := a.k8sClient.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if !k8sErrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func (a airflowClient) deleteSecret(ctx context.Context, name, namespace string) error {
	if a.dryRun {
		a.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	if err := a.k8sClient.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if !k8sErrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}
