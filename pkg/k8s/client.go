package k8s

import (
	"fmt"
	"net/url"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	kcapi "k8s.io/client-go/tools/clientcmd/api"
)

type KubeConfig struct {
	name     string
	contents []byte
}

// Stolen from, with some modifications:
// - https://github.com/kubernetes-sigs/controller-runtime/blob/main/pkg/internal/testing/controlplane/kubectl.go#L42
func (k *KubeConfig) FromREST(cfg *rest.Config) error {
	kubeConfig := kcapi.NewConfig()

	protocol := "https"
	if !rest.IsConfigTransportTLS(*cfg) {
		protocol = "http"
	}

	// cfg.Host is a URL, so we need to parse it so we can properly append the API path
	baseURL, err := url.Parse(cfg.Host)
	if err != nil {
		return fmt.Errorf("unable to interpret config's host value as a URL: %w", err)
	}

	kubeConfig.Clusters[k.name] = &kcapi.Cluster{
		Server:                   (&url.URL{Scheme: protocol, Host: baseURL.Host, Path: cfg.APIPath}).String(),
		CertificateAuthorityData: cfg.CAData,
	}
	kubeConfig.AuthInfos[k.name] = &kcapi.AuthInfo{
		// try to cover all auth strategies that aren't plugins
		ClientCertificateData: cfg.CertData,
		ClientKeyData:         cfg.KeyData,
		Token:                 cfg.BearerToken,
		Username:              cfg.Username,
		Password:              cfg.Password,
	}
	kcCtx := kcapi.NewContext()
	kcCtx.Cluster = k.name
	kcCtx.AuthInfo = k.name
	kubeConfig.Contexts[k.name] = kcCtx
	kubeConfig.CurrentContext = k.name

	contents, err := clientcmd.Write(*kubeConfig)
	if err != nil {
		return fmt.Errorf("unable to serialize kubeconfig file: %w", err)
	}

	k.contents = contents

	return nil
}

func (k *KubeConfig) Name() string {
	return k.name
}

func (k *KubeConfig) Contents() []byte {
	return k.contents
}

func NewKubeConfig(name string) *KubeConfig {
	return &KubeConfig{
		name: name,
	}
}
