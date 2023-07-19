package k8s

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func CreateClientset(inCluster bool) (*kubernetes.Clientset, error) {
	config, err := createKubeConfig(inCluster)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

func createKubeConfig(inCluster bool) (*rest.Config, error) {
	if inCluster {
		return rest.InClusterConfig()
	}

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = filepath.Join(homedir.HomeDir(), ".kube", "config")
	}

	// use the current context in kubeconfig
	return clientcmd.BuildConfigFromFlags("", kubeconfig)
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
