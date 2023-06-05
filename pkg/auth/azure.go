package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc"
	"github.com/golang-jwt/jwt/v4"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"
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
	IsAdmin     bool
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

type AzureGroupsWithIDResponse struct {
	Groups []AzureGroupWithID `json:"value"`
}

type AzureGroupWithID struct {
	DisplayName string `json:"displayName"`
	ID          string `json:"id"`
	Mail        string `json:"mail"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
}

var ErrAzureTokenExpired = fmt.Errorf("token expired")

const (
	AzureUsersEndpoint  = "https://graph.microsoft.com/v1.0/users"
	AzureGroupsEndpoint = "https://graph.microsoft.com/v1.0/groups"
)

func New(dryRun bool, clientID, clientSecret, tenantID, hostname string, log *logrus.Entry) *Azure {
	if dryRun {
		return nil
	}

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

	azureToken, err := jwt.ParseWithClaims(token, &claims, jwtValidator)
	if err != nil {
		return nil, err
	}
	if !azureToken.Valid {
		return nil, ErrAzureTokenExpired
	}

	return &User{
		Name:    claims["name"].(string),
		Email:   strings.ToLower(claims["preferred_username"].(string)),
		Expires: time.Unix(int64(claims["exp"].(float64)), 0),
	}, nil
}

func (a *Azure) UserExistsInAzureAD(user string) error {
	type usersResponse struct {
		Value []struct {
			Email string `json:"userPrincipalName"`
		} `json:"value"`
	}

	token, err := a.getBearerTokenForApplication()
	if err != nil {
		return err
	}

	r, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%v?$filter=startswith(userPrincipalName,'%v')", AzureUsersEndpoint, user), nil)
	if err != nil {
		return err
	}
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %v", token))

	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}

	res, err := httpClient.Do(r)
	if err != nil {
		return err
	}

	resBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	var users usersResponse
	if err := json.Unmarshal(resBytes, &users); err != nil {
		return err
	}

	switch len(users.Value) {
	case 0:
		return fmt.Errorf("no user exists in aad with email %v", user)
	case 1:
		return nil
	default:
		return fmt.Errorf("multiple users exist in aad for email %v", user)
	}
}

func (a *Azure) getBearerTokenForApplication() (string, error) {
	form := url.Values{}
	form.Add("client_id", a.clientID)
	form.Add("client_secret", a.clientSecret)
	form.Add("scope", "https://graph.microsoft.com/.default")
	form.Add("grant_type", "client_credentials")

	req, err := http.NewRequest(http.MethodPost, endpoints.AzureAD(a.tenantID).TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}

	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}

	response, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}

	var tokenResponse TokenResponse
	if err := json.NewDecoder(response.Body).Decode(&tokenResponse); err != nil {
		return "", err
	}

	return tokenResponse.AccessToken, nil
}

func (a *Azure) GetGroupID(groupMail string) (string, error) {
	token, err := a.getBearerTokenForApplication()
	if err != nil {
		return "", err
	}

	params := url.Values{}
	params.Add("$select", "id,displayName,mail")
	params.Add("$filter", fmt.Sprintf("mail eq '%v'", groupMail))

	req, err := http.NewRequest(http.MethodGet,
		AzureGroupsEndpoint+"?"+params.Encode(),
		nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %v", token))

	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}

	response, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}

	var groupsResponse AzureGroupsWithIDResponse
	if err := json.NewDecoder(response.Body).Decode(&groupsResponse); err != nil {
		return "", err
	}

	if len(groupsResponse.Groups) > 0 {
		return groupsResponse.Groups[0].ID, nil
	} else {
		return "", errors.New("Group not found by the mail")
	}
}
