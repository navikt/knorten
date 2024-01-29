package config_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"gopkg.in/yaml.v3"

	"github.com/nais/knorten/pkg/config"
	"github.com/spf13/afero"
)

var update = flag.Bool("update", false, "update golden files")

func newFsWithConfig(t *testing.T, path string, data []byte) afero.Fs { //nolint: ireturn
	t.Helper()

	fs := afero.NewMemMapFs()

	err := afero.WriteFile(fs, path, data, 0o644)
	t.Errorf("write file: %v", err)

	return fs
}

func newFakeConfig() config.Config {
	return config.Config{
		Oauth: config.Oauth{
			ClientID:     "fake-client-id",
			ClientSecret: "fake-client-secret",
			TenantID:     "fake-tenant-id",
		},
		GCP: config.GCP{
			Project: "knorten",
			Region:  "europe-north1",
			Zone:    "europe-north1-b",
		},
		Cookies: config.Cookies{
			Redirect: config.CookieSettings{
				Name:     "redirect",
				MaxAge:   3600,
				Path:     "/",
				Domain:   "localhost",
				SameSite: "Lax",
				Secure:   false,
				HttpOnly: true,
			},
			OauthState: config.CookieSettings{
				Name:     "oauth_state",
				MaxAge:   2400,
				Path:     "/",
				Domain:   "knorten.knada.io",
				SameSite: "Strict",
				Secure:   true,
				HttpOnly: true,
			},
			Session: config.CookieSettings{
				Name:     "session",
				MaxAge:   0,
				Path:     "/",
				Domain:   "",
				SameSite: "Lax",
				Secure:   true,
				HttpOnly: true,
			},
		},
		Helm: config.Helm{
			AirflowChartVersion: "1.10.0",
			JupyterChartVersion: "2.0.0",
		},
		Server: config.Server{
			Hostname: "localhost",
			Port:     8080,
		},
		Postgres: config.Postgres{
			Host:         "localhost",
			Port:         5432,
			UserName:     "postgres",
			Password:     "postgres",
			SSLMode:      "disable",
			DatabaseName: "knorten",
		},
		DBEncKey:   "jegersekstentegn",
		AdminGroup: "nada@nav.no",
		SessionKey: "test-session",
		LoginPage:  "http://localhost:8080/",
		DryRun:     false,
		InCluster:  false,
	}
}

func updateGoldenFiles(t *testing.T, filePath string, cfg config.Config) []byte {
	t.Helper()

	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Errorf("marshal config: %v", err)
	}

	err = afero.WriteFile(afero.NewOsFs(), filePath, data, 0o644)
	if err != nil {
		t.Errorf("write golden file: %v", err)
	}

	return data
}

func TestLoad(t *testing.T) {
	if *update {
		t.Log("Updating golden files")
		updateGoldenFiles(t, "testdata/config.yaml", newFakeConfig())
		t.Log("Done updating golden files")

		return
	}

	testCases := []struct {
		name      string
		config    string
		path      string
		envPrefix string
		loader    config.Loader
		envs      map[string]string
		expect    config.Config
		expectErr bool
	}{
		{
			name:      "Standard config",
			config:    "config",
			path:      "testdata",
			loader:    config.NewFileSystemLoader(afero.NewOsFs()),
			expect:    newFakeConfig(),
			expectErr: false,
		},
		{
			name:   "Standard config with env overrides",
			config: "config",
			path:   "testdata",
			loader: config.NewFileSystemLoader(afero.NewOsFs()),
			expect: func() config.Config {
				cfg := newFakeConfig()
				cfg.AdminGroup = "something_super_random"

				return cfg
			}(),
			envs: map[string]string{
				"ADMIN_GROUP": "something_super_random",
			},
		},
		{
			name:      "Standard config with env prefix overrides",
			config:    "config",
			path:      "testdata",
			envPrefix: "knorten",
			loader:    config.NewFileSystemLoader(afero.NewOsFs()),
			expect: func() config.Config {
				cfg := newFakeConfig()
				cfg.AdminGroup = "something_super_random"

				return cfg
			}(),
			envs: map[string]string{
				"KNORTEN_ADMIN_GROUP": "something_super_random",
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.envs {
				t.Setenv(k, v)
			}

			cfg, err := tc.loader.Load(tc.config, tc.path, tc.envPrefix)
			if err != nil && !tc.expectErr {
				t.Errorf("unexpected error: %v", err)
			}

			if err == nil && tc.expectErr {
				t.Errorf("expected error, got none")
			}

			if !tc.expectErr {
				if diff := cmp.Diff(tc.expect, cfg); diff != "" {
					t.Errorf("mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

func getWorkingDir(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Errorf("get working dir: %v", err)
	}

	return wd
}

func TestProcessConfigPath(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		path      string
		expect    config.FileParts
		expectErr bool
	}{
		{
			name: "Valid config path",
			path: "testdata/config.yaml",
			expect: config.FileParts{
				FileName: "config",
				Path:     filepath.Join(getWorkingDir(t), "testdata"),
			},
		},
		{
			name:      "Invalid extension",
			path:      "testdata/config.json",
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := config.ProcessConfigPath(tc.path)
			if err != nil && !tc.expectErr {
				t.Errorf("unexpected error: %v", err)
			}

			if err == nil && tc.expectErr {
				t.Errorf("expected error, got none")
			}

			if !tc.expectErr {
				if diff := cmp.Diff(tc.expect, got); diff != "" {
					t.Errorf("mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}
