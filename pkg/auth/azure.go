package auth

import (
	"context"
	"encoding/json"
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

type MemberOfResponse struct {
	Groups []MemberOfGroup `json:"value"`
}

type MemberOfGroup struct {
	DisplayName string   `json:"displayName"`
	Mail        string   `json:"mail"`
	GroupTypes  []string `json:"groupTypes"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
}

var ErrAzureTokenExpired = fmt.Errorf("token expired")

const (
	AzureGraphMemberOfEndpoint = "https://graph.microsoft.com/v1.0/me/memberOf/microsoft.graph.group?$select=mail"
	AzureUsersEndpoint         = "https://graph.microsoft.com/v1.0/users"
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

func (a *Azure) IdentForEmail(email string) (string, error) {
	type identResponse struct {
		Ident string `json:"onPremisesSamAccountName"`
	}

	token, err := a.getBearerTokenForApplication()
	if err != nil {
		return "", err
	}

	r, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%v/%v?$select=onPremisesSamAccountName", AzureUsersEndpoint, email), nil)
	if err != nil {
		return "", err
	}
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %v", token))

	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}

	res, err := httpClient.Do(r)
	if err != nil {
		return "", err
	}

	resBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	var identRes identResponse
	if err := json.Unmarshal(resBytes, &identRes); err != nil {
		return "", err
	}

	fmt.Println(string(resBytes))

	fmt.Println(identRes)

	if identRes.Ident == "" {
		return "", fmt.Errorf("unable to get user ident for email %v", email)
	}

	return strings.ToLower(identRes.Ident), nil
}

func (a *Azure) GroupsForUser(token, email string) ([]MemberOfGroup, error) {
	bearerToken, err := a.getBearerTokenOnBehalfOfUser(token)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, AzureGraphMemberOfEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %v", bearerToken))

	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}

	response, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	var body []byte
	_, err = response.Body.Read(body)
	if err != nil {
		return nil, err
	}

	var memberOfResponse MemberOfResponse
	if err := json.NewDecoder(response.Body).Decode(&memberOfResponse); err != nil {
		return nil, err
	}

	return memberOfResponse.Groups, nil
}

func contains(groups []MemberOfGroup, email string) bool {
	for _, group := range groups {
		if strings.EqualFold(group.Mail, email) {
			return true
		}
	}
	return false
}

func (a *Azure) UserInGroup(token string, userEmail, groupEmail string) (bool, error) {
	groups, err := a.GroupsForUser(token, userEmail)
	if err != nil {
		return false, err
	}

	return contains(groups, groupEmail), nil
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

func (a *Azure) getBearerTokenOnBehalfOfUser(token string) (string, error) {
	form := url.Values{}
	form.Add("client_id", a.clientID)
	form.Add("client_secret", a.clientSecret)
	form.Add("scope", "https://graph.microsoft.com/.default")
	form.Add("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Add("requested_token_use", "on_behalf_of")
	form.Add("assertion", token)

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

	logrus.Debugf("Successfully retrieved on-behalf-of token")
	return tokenResponse.AccessToken, nil
}
