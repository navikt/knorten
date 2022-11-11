package k8s

import (
	"context"
	"errors"
	"os"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

type Client struct {
	clientSet *kubernetes.Clientset
	dryRun    bool
	inCluster bool
}

func New(dryRun, inCluster bool) (*Client, error) {
	client := &Client{
		dryRun: dryRun,
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
				"cert-secret-jupyterhub": "include",
				"cert-secret-airflow":    "include",
				"azureadapp-secret":      "include",
				"smtp-secret":            "include",
				"slack-secret":           "include",
				"github-app-secret":      "include",
				"ghcr-secret":            "include",
				"ca-bundle-cm":           "include",
			},
		},
	}

	_, err := c.clientSet.CoreV1().Namespaces().Create(ctx, nsSpec, metav1.CreateOptions{})
	if err != nil {
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
