package helm

import (
	"bytes"
	"context"
	"fmt"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"
	"io"
	"os"
	"sigs.k8s.io/yaml"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultHelmDriver is set to secrets, which is the default
	// for Helm 3: https://helm.sh/docs/topics/advanced/#storage-backends
	DefaultHelmDriver = "secrets"
	// DefaultHelmLockExt is the extension used to create a file lock
	DefaultHelmLockExt = ".lock"
	// DefaultHelmLockTimeout is the default timeout in seconds
	DefaultHelmLockTimeout = 120 * time.Second
)

type ErrRollback struct {
	msg string
	err error
}

func (e *ErrRollback) Error() string {
	return fmt.Sprintf("%s: %v", e.msg, e.err)
}

func (e *ErrRollback) Unwrap() error {
	return e.err
}

func NewErrRollback(err error) *ErrRollback {
	return &ErrRollback{
		msg: "rollback due to error",
		err: err,
	}
}

// Chart contains the state for installing a chart
type Chart struct {
	RepositoryName string
	RepositoryURL  string

	ReleaseName string
	Version     string
	Chart       string
	Namespace   string

	Timeout time.Duration

	Values interface{}
}

// Marshaller provides an interface for returning YAML
type Marshaller interface {
	MarshalYAML() ([]byte, error)
}

type ApplyOpts struct {
	ReleaseName string
	Namespace   string
}

type Applier interface {
	// Apply will either install or upgrade a chart depending on the
	// state of the release in the cluster
	Apply(ctx context.Context, loader ChartLoader, opts *ApplyOpts) error
}

type DeleteOpts struct {
	ReleaseName string
	Namespace   string
}

type Deleter interface {
	// Delete will remove a release from the cluster
	Delete(ctx context.Context, opts *DeleteOpts) error
}

type RollbackOpts struct {
	ReleaseName string
	Namespace   string
}

type Rollbacker interface {
	Rollback(ctx context.Context, opts *RollbackOpts) error
}

type ChartLoader interface {
	// Load a chart from a source
	Load(ctx context.Context) (*chart.Chart, error)
}

type Operations interface {
	Applier
	Deleter
	Rollbacker
}

type Helm struct {
	config *Config
}

func NewHelm(config *Config) *Helm {
	return &Helm{
		config: config,
	}
}

var _ Operations = &Helm{}

func (h *Helm) Apply(ctx context.Context, loader ChartLoader, opts *ApplyOpts) error {
	restoreFn, err := EstablishEnv(h.config.ToHelmEnvs())
	if err != nil {
		return fmt.Errorf("establishing helm env: %w", err)
	}

	defer func() {
		_ = restoreFn()
	}()

	settings := cli.New()
	settings.SetNamespace(opts.Namespace)
	actionConfig := new(action.Configuration)

	debug := func(format string, v ...interface{}) {
		if h.config.Debug {
			_, _ = fmt.Fprintf(h.config.Err, format, v...)
		}
	}

	err = actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), DefaultHelmDriver, debug)
	if err != nil {
		return fmt.Errorf("initializing helm action config: %w", err)
	}

	exists, err := releaseExists(actionConfig, opts.ReleaseName)
	if err != nil {
		return fmt.Errorf("checking if release exists: %w", err)
	}

	ch, err := loader.Load(ctx)
	if err != nil {
		return fmt.Errorf("loading values: %w", err)
	}

	if !exists {
		// Release does not exist, so we install
		installClient := action.NewInstall(actionConfig)
		installClient.Namespace = opts.Namespace
		installClient.ReleaseName = opts.ReleaseName
		installClient.Timeout = timeout
		installClient.CreateNamespace = true
		installClient.DryRun = h.config.DryRun
		if h.config.Debug {
			installClient.PostRenderer = NewStreamRenderer(h.config.Err)
		}

		_, err = installClient.RunWithContext(ctx, ch, ch.Values)
		if err != nil {
			return fmt.Errorf("installing release: %w", err)
		}

		// We are done here, so lets return
		return nil
	}

	// Release exists, so we upgrade
	upgradeClient := action.NewUpgrade(actionConfig)
	upgradeClient.Namespace = opts.Namespace
	upgradeClient.Timeout = timeout
	upgradeClient.DryRun = h.config.DryRun

	if h.config.Debug {
		upgradeClient.PostRenderer = NewStreamRenderer(h.config.Err)
	}

	_, err = upgradeClient.RunWithContext(ctx, opts.ReleaseName, ch, ch.Values)
	if err != nil {
		return NewErrRollback(fmt.Errorf("upgrading release: %w", err))
	}

	return nil
}

func (h *Helm) Delete(_ context.Context, opts *DeleteOpts) error {
	restoreFn, err := EstablishEnv(h.config.ToHelmEnvs())
	if err != nil {
		return fmt.Errorf("establishing helm env: %w", err)
	}

	defer func() {
		_ = restoreFn()
	}()

	debug := func(format string, v ...interface{}) {
		if h.config.Debug {
			_, _ = fmt.Fprintf(h.config.Err, format, v...)
		}
	}

	settings := cli.New()
	settings.SetNamespace(opts.Namespace)

	actionConfig := new(action.Configuration)

	err = actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), DefaultHelmDriver, debug)
	if err != nil {
		return fmt.Errorf("initializing helm action config: %w", err)
	}

	exists, err := releaseExists(actionConfig, opts.ReleaseName)
	if err != nil {
		return fmt.Errorf("checking if release exists: %w", err)
	}

	if !exists {
		// Already deleted
		return nil
	}

	uninstallClient := action.NewUninstall(actionConfig)
	_, err = uninstallClient.Run(opts.ReleaseName)
	if err != nil {
		return fmt.Errorf("uninstalling release: %w", err)
	}

	return nil
}

func (h *Helm) Rollback(_ context.Context, opts *RollbackOpts) error {
	restoreFn, err := EstablishEnv(h.config.ToHelmEnvs())
	if err != nil {
		return fmt.Errorf("establishing helm env: %w", err)
	}

	defer func() {
		_ = restoreFn()
	}()

	settings := cli.New()
	settings.SetNamespace(opts.Namespace)
	actionConfig := new(action.Configuration)

	debug := func(format string, v ...interface{}) {
		if h.config.Debug {
			_, _ = fmt.Fprintf(h.config.Err, format, v...)
		}
	}

	err = actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), "secret", debug)
	if err != nil {
		return fmt.Errorf("initializing helm action config: %w", err)
	}

	version, err := lastSuccessfulHelmRelease(opts.ReleaseName, actionConfig)
	if err != nil {
		return fmt.Errorf("getting last successful helm release: %w", err)
	}

	rollbackClient := action.NewRollback(actionConfig)
	rollbackClient.Version = version
	if err := rollbackClient.Run(opts.ReleaseName); err != nil {
		return fmt.Errorf("rolling back release: %w", err)
	}

	return nil
}

// RestoreEnvFn can be deferred in the calling function
// and will return the environment to its original state
type RestoreEnvFn func() error

type Config struct {
	KubeContext      string
	Debug            bool
	RepositoryConfig string

	DryRun bool

	Out io.Writer
	Err io.Writer
}

func (c *Config) ToHelmEnvs() map[string]string {
	return map[string]string{
		"HELM_KUBECONTEXT":       c.KubeContext,
		"HELM_DEBUG":             strconv.FormatBool(c.Debug),
		"HELM_REPOSITORY_CONFIG": c.RepositoryConfig,
	}
}

type StreamRender struct {
	out io.Writer
}

func (r *StreamRender) Run(renderedManifests *bytes.Buffer) (*bytes.Buffer, error) {
	_, err := fmt.Fprintf(r.out, "Rendered output: %s\n", renderedManifests.String())
	if err != nil {
		return renderedManifests, err
	}

	return renderedManifests, nil
}

func NewStreamRenderer(out io.Writer) *StreamRender {
	return &StreamRender{
		out: out,
	}
}

// EstablishEnv provides functionality for setting a safe environment,
// this is required, because helm for some reason, loves fetching
// everything from environment variables
func EstablishEnv(envs map[string]string) (RestoreEnvFn, error) {
	originalEnvVars := os.Environ()
	os.Clearenv()

	fn := func() error {
		originalEnvVars := originalEnvVars

		os.Clearenv()

		originalSplit := SplitEnv(originalEnvVars)

		for k, v := range originalSplit {
			err := os.Setenv(k, v)
			if err != nil {
				return fmt.Errorf("restoring environment: %w", err)
			}
		}

		return nil
	}

	for key, val := range envs {
		err := os.Setenv(key, val)
		if err != nil {
			return fn, fmt.Errorf("setting environment: %w", err)
		}
	}

	return fn, nil
}

// SplitEnv returns the split envvars
func SplitEnv(envs []string) map[string]string {
	out := map[string]string{}
	numberOfResultingSubstrings := 2

	for _, envVar := range envs {
		e := strings.SplitN(envVar, "=", numberOfResultingSubstrings)

		var key, val string

		switch len(e) {
		case 0:
			continue
		case 1:
			key = e[0]
			val = ""
		case 2: //nolint: gomnd
			key = e[0]
			val = e[1]
		}

		out[key] = val
	}

	return out
}

// UnmarshalToValues takes a byte slice and unmarshalls it into a map, which
// is what Helm's API expects
func UnmarshalToValues(data []byte) (map[string]interface{}, error) {
	var values map[string]interface{}

	err := yaml.Unmarshal(data, &values)
	if err != nil {
		return nil, err
	}

	return values, nil
}
