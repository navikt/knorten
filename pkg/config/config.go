package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-ozzo/ozzo-validation/v4/is"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"github.com/mitchellh/mapstructure"

	"github.com/spf13/afero"
	"github.com/spf13/viper"
)

const (
	defaultExtension = "yaml"
	defaultTagName   = "yaml"
)

type Loader interface {
	Load(name, path, envPrefix string) (Config, error)
}

type Config struct {
	Oauth      Oauth    `yaml:"oauth"`
	GCP        GCP      `yaml:"gcp"`
	Cookies    Cookies  `yaml:"cookies"`
	Helm       Helm     `yaml:"helm"`
	Server     Server   `yaml:"server"`
	Postgres   Postgres `yaml:"postgres"`
	DBEncKey   string   `yaml:"db_enc_key"`
	AdminGroup string   `yaml:"admin_group"`
	SessionKey string   `yaml:"session_key"`
	LoginPage  string   `yaml:"login_page"`
	DryRun     bool     `yaml:"dry_run"`
	InCluster  bool     `yaml:"in_cluster"`
}

func (c Config) Validate() error {
	return validation.ValidateStruct(&c,
		validation.Field(&c.Oauth, validation.Required),
		validation.Field(&c.GCP, validation.Required),
		validation.Field(&c.Cookies, validation.Required),
		validation.Field(&c.Helm, validation.Required),
		validation.Field(&c.Server, validation.Required),
		validation.Field(&c.Postgres, validation.Required),
		validation.Field(&c.DBEncKey, validation.Required),
		validation.Field(&c.LoginPage, validation.Required),
		validation.Field(&c.AdminGroup, validation.Required),
		validation.Field(&c.SessionKey, validation.Required),
	)
}

type Postgres struct {
	UserName     string `yaml:"user_name"`
	Password     string `yaml:"password"`
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	DatabaseName string `yaml:"database_name"`
	SSLMode      string `yaml:"ssl_mode"`
}

func (p Postgres) Validate() error {
	return validation.ValidateStruct(&p,
		validation.Field(&p.UserName, validation.Required),
		validation.Field(&p.Password, validation.Required),
		validation.Field(&p.Host, validation.Required, is.Host),
		validation.Field(&p.Port, validation.Required, is.Port),
		validation.Field(&p.DatabaseName, validation.Required),
		validation.Field(&p.SSLMode, validation.Required),
	)
}

func (p Postgres) ConnectionString() string {
	return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=%s",
		p.UserName,
		p.Password,
		p.Host,
		p.Port,
		p.DatabaseName,
		p.SSLMode,
	)
}

type Server struct {
	Hostname string `yaml:"hostname"`
	Port     int    `yaml:"port"`
}

func (s Server) Validate() error {
	return validation.ValidateStruct(&s,
		validation.Field(&s.Hostname, validation.Required, is.Host),
		validation.Field(&s.Port, validation.Required, is.Port),
	)
}

type Oauth struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	TenantID     string `yaml:"tenant_id"`
}

func (o Oauth) Validate() error {
	return validation.ValidateStruct(&o,
		validation.Field(&o.ClientID, validation.Required),
		validation.Field(&o.ClientSecret, validation.Required),
		validation.Field(&o.TenantID, validation.Required),
	)
}

type GCP struct {
	Project string `yaml:"project"`
	Region  string `yaml:"region"`
	Zone    string `yaml:"zone"`
}

func (g GCP) Validate() error {
	return validation.ValidateStruct(&g,
		validation.Field(&g.Project, validation.Required),
		// Valid regions and zones:
		// - https://cloud.google.com/compute/docs/regions-zones
		validation.Field(&g.Region, validation.Required, validation.In("europe-north1")),
		validation.Field(&g.Zone, validation.Required, validation.In("europe-north1-a", "europe-north1-b", "europe-north1-c")),
	)
}

type Helm struct {
	AirflowChartVersion string `yaml:"airflow_chart_version"`
	JupyterChartVersion string `yaml:"jupyter_chart_version"`
}

func (h Helm) Validate() error {
	return validation.ValidateStruct(&h,
		validation.Field(&h.AirflowChartVersion, validation.Required),
		validation.Field(&h.JupyterChartVersion, validation.Required),
	)
}

type Cookies struct {
	Redirect   CookieSettings `yaml:"redirect"`
	OauthState CookieSettings `yaml:"oauth_state"`
	Session    CookieSettings `yaml:"session"`
}

func (c Cookies) Validate() error {
	return validation.ValidateStruct(&c,
		validation.Field(&c.Redirect, validation.Required),
		validation.Field(&c.OauthState, validation.Required),
		validation.Field(&c.Session, validation.Required),
	)
}

type CookieSettings struct {
	Name     string `yaml:"name"`
	MaxAge   int    `yaml:"max_age"`
	Path     string `yaml:"path"`
	Domain   string `yaml:"domain"`
	SameSite string `yaml:"same_site"`
	Secure   bool   `yaml:"secure"`
	HttpOnly bool   `yaml:"http_only"`
}

func (c CookieSettings) Validate() error {
	return validation.ValidateStruct(&c,
		validation.Field(&c.Name, validation.Required),
		validation.Field(&c.MaxAge, validation.Required),
		validation.Field(&c.Path, validation.Required),
		validation.Field(&c.Domain, validation.Required, is.Host),
		// Valid SameSite values:
		// - https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie#samesitesamesite-value
		validation.Field(&c.SameSite, validation.Required, validation.In("Strict", "Lax", "None")),
	)
}

func NewDefault() Config {
	return Config{
		AdminGroup: "nada@nav.no",
	}
}

type FileParts struct {
	FileName string
	Path     string
}

func ProcessConfigPath(configFile string) (FileParts, error) {
	absolutePath, err := filepath.Abs(configFile)
	if err != nil {
		return FileParts{}, fmt.Errorf("convert to absolute path: %w", err)
	}

	// Extract file name and extension
	fileName := filepath.Base(absolutePath)
	path := filepath.Dir(absolutePath)
	extension := filepath.Ext(fileName)

	if strings.ReplaceAll(strings.ToLower(extension), ".", "") != defaultExtension {
		return FileParts{}, fmt.Errorf("config file must have extension %s, got: %s", defaultExtension, extension)
	}

	return FileParts{
		FileName: fileName[:len(fileName)-len(extension)],
		Path:     path,
	}, nil
}

func NewFileSystemLoader(fs afero.Fs) *FileSystemLoader {
	return &FileSystemLoader{fs}
}

type FileSystemLoader struct {
	afero.Fs
}

func (fs *FileSystemLoader) Load(name, path, envPrefix string) (Config, error) {
	v := viper.New()

	v.AddConfigPath(path)
	v.SetConfigName(name)
	v.SetConfigType(defaultExtension)

	v.AutomaticEnv()
	v.SetEnvPrefix(envPrefix)

	err := v.ReadInConfig()
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var config Config

	err = v.Unmarshal(&config, func(cfg *mapstructure.DecoderConfig) {
		cfg.TagName = defaultTagName // We use yaml tags in the config structs so we can marshal to yaml
	})
	if err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}

	return config, nil
}
