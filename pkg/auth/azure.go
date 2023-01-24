package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/coreos/go-oidc"
	"github.com/golang-jwt/jwt/v4"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

type OauthConfig struct {
	ClientID     string
	ClientSecret string
	TenantID     string
	Hostname     string
}

type Session struct {
	Email       string `json:"preferred_username"`
	Name        string `json:"name"`
	AccessToken string
	Token       string
	Expires     time.Time
}

type Azure struct {
	oauth2.Config

	clientID     string
	clientSecret string
	tenantID     string
	hostname     string
	provider     *oidc.Provider
	log          *logrus.Entry
}

type User struct {
	Name    string
	Email   string
	Expires time.Time
}

func New(clientID, clientSecret, tenantID, hostname string, log *logrus.Entry) *Azure {
	provider, err := oidc.NewProvider(context.Background(), fmt.Sprintf("https://login.microsoftonline.com/%v/v2.0", tenantID))
	if err != nil {
		panic(err)
	}

	a := &Azure{
		clientID:     clientID,
		clientSecret: clientSecret,
		tenantID:     tenantID,
		hostname:     hostname,
		provider:     provider,
		log:          log,
	}

	a.setupOAuth2()
	return a
}

func (a *Azure) setupOAuth2() {
	var callbackURL string
	if a.hostname == "localhost" {
		callbackURL = "http://localhost:8080/oauth2/callback"
	} else {
		callbackURL = fmt.Sprintf("https://%v/oauth2/callback", a.hostname)
	}

	a.Config = oauth2.Config{
		ClientID:     a.clientID,
		ClientSecret: a.clientSecret,
		Endpoint:     a.provider.Endpoint(),
		RedirectURL:  callbackURL,
		Scopes:       []string{"openid", fmt.Sprintf("%s/.default", a.clientID)},
	}
}

func (a *Azure) KeyDiscoveryURL() string {
	return fmt.Sprintf("https://login.microsoftonline.com/%s/discovery/v2.0/keys", a.tenantID)
}

func (a *Azure) Verify(ctx context.Context, rawIDToken string) (*oidc.IDToken, error) {
	return a.provider.Verifier(&oidc.Config{ClientID: a.clientID}).Verify(ctx, rawIDToken)
}

func (a *Azure) FetchCertificates() (map[string]CertificateList, error) {
	discoveryURL := a.KeyDiscoveryURL()
	azureKeyDiscovery, err := DiscoverURL(discoveryURL)
	if err != nil {
		return nil, err
	}

	azureCertificates, err := azureKeyDiscovery.Map()
	if err != nil {
		return nil, err
	}
	return azureCertificates, nil
}

func (a *Azure) ValidateUser(certificates map[string]CertificateList, token string) (*User, error) {
	var claims jwt.MapClaims

	jwtValidator := JWTValidator(certificates, a.clientID)

	_, err := jwt.ParseWithClaims(token, &claims, jwtValidator)
	if err != nil {
		return nil, err
	}

	return &User{
		Name:    claims["name"].(string),
		Email:   strings.ToLower(claims["preferred_username"].(string)),
		Expires: time.Unix(int64(claims["exp"].(float64)), 0),
	}, nil
}
