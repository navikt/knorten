package k8s

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func NameToNamespace(name string) string {
	if strings.HasPrefix(name, "team-") {
		return name
	} else if strings.HasPrefix(name, "team") {
		return strings.Replace(name, "team", "team-", 1)
	} else {
		return fmt.Sprintf("team-%v", name)
	}
}

type Client struct {
	clientSet  *kubernetes.Clientset
	dryRun     bool
	inCluster  bool
	gcpProject string
	gcpRegion  string
	log        *logrus.Entry
}

func New(log *logrus.Entry, dryRun, inCluster bool, gcpProject, gcpRegion string) (*Client, error) {
	client := &Client{
		dryRun:     dryRun,
		gcpProject: gcpProject,
		gcpRegion:  gcpRegion,
		log:        log,
	}

	config, err := createConfig(inCluster)
	if err != nil {
		return nil, err
	}

	client.clientSet, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (c *Client) CreateTeamNamespace(ctx context.Context, name string) error {
	if c.dryRun {
		return nil
	}

	nsSpec := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"cert-secret-jupyterhub":     "include",
				"cert-secret-airflow":        "include",
				"azureadapp-secret":          "include",
				"smtp-secret":                "include",
				"slack-secret":               "include",
				"github-app-secret":          "include",
				"ghcr-secret":                "include",
				"ca-bundle-cm":               "include",
				"airflow-webserver-config":   "include",
				"airflow-auth-config":        "include",
				"airflow-global-envs-config": "include",
			},
		},
	}

	_, err := c.clientSet.CoreV1().Namespaces().Create(ctx, nsSpec, metav1.CreateOptions{})
	if err != nil {
		c.log.WithError(err).Error("creating team namespace")
		return err
	}

	return nil
}

func (c *Client) DeleteTeamNamespace(ctx context.Context, namespace string) error {
	if c.dryRun {
		return nil
	}

	err := c.clientSet.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
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

func (c *Client) CreateTeamServiceAccount(ctx context.Context, teamID, namespace string) error {
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

	_, err := c.clientSet.CoreV1().ServiceAccounts(namespace).Create(ctx, saSpec, metav1.CreateOptions{})
	if err != nil {
		c.log.WithError(err).Error("creating team service account")
		return err
	}

	return nil
}

func (c *Client) CreateCloudSQLProxy(ctx context.Context, name, teamID, namespace, dbInstance string) error {
	if c.dryRun {
		c.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	port := int32(5432)

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
					ServiceAccountName: teamID,
					Containers: []v1.Container{
						{
							Name:  "cloudsql-proxy",
							Image: "gcr.io/cloudsql-docker/gce-proxy:1.29.0-alpine",
							Ports: []v1.ContainerPort{
								{
									Protocol:      v1.ProtocolTCP,
									ContainerPort: port,
								},
							},
							Command: []string{
								"/cloud_sql_proxy",
								"-term_timeout=30s",
								fmt.Sprintf("-instances=%v:%v:%v=tcp:0.0.0.0:%v", c.gcpProject, c.gcpRegion, dbInstance, port),
							},
						},
					},
				},
			},
		},
	}

	_, err := c.clientSet.AppsV1().Deployments(namespace).Create(ctx, deploySpec, metav1.CreateOptions{})
	if err != nil {
		c.log.WithError(err).Error("creating cloudsql proxy deployment")
		return err
	}

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

	_, err = c.clientSet.CoreV1().Services(namespace).Create(ctx, serviceSpec, metav1.CreateOptions{})
	if err != nil {
		c.log.WithError(err).Error("creating cloudsql proxy service")
		return err
	}

	return nil
}

func (c *Client) DeleteCloudSQLProxy(ctx context.Context, name, namespace string) error {
	if c.dryRun {
		c.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	if err := c.clientSet.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if k8sErrors.IsNotFound(err) {
			c.log.Infof("delete deployment: deployment %v in namespace %v does not exist", name, namespace)
			return nil
		}
		c.log.WithError(err).Error("deleting cloudsql proxy deployment")
		return err
	}

	if err := c.clientSet.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if k8sErrors.IsNotFound(err) {
			c.log.Infof("delete service: service %v in namespace %v does not exist", name, namespace)
			return nil
		}
		c.log.WithError(err).Error("deleting cloudsql proxy service")
		return err
	}

	return nil
}

func (c *Client) CreateSecret(ctx context.Context, name, namespace string, data map[string]string) error {
	if c.dryRun {
		c.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		StringData: data,
	}

	_, err := c.clientSet.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		c.log.WithError(err).Error("creating secret %v in namespace %v", name, namespace)
		return err
	}

	return nil
}

func (c *Client) DeleteSecret(ctx context.Context, name, namespace string) error {
	if c.dryRun {
		c.log.Infof("NOOP: Running in dry run mode")
		return nil
	}

	if err := c.clientSet.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if k8sErrors.IsNotFound(err) {
			c.log.Infof("delete secret: secret %v in namespace %v does not exist", name, namespace)
			return nil
		}
		c.log.WithError(err).Error("deleting secret %v in namespace %v", name, namespace)
		return err
	}

	return nil
}

func createConfig(inCluster bool) (*rest.Config, error) {
	if inCluster {
		return rest.InClusterConfig()
	}

	configPath := os.Getenv("KUBECONFIG")
	if configPath == "" {
		return nil, errors.New("KUBECONFIG env not set")
	}

	return clientcmd.BuildConfigFromFlags("", configPath)
}
