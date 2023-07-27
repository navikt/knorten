package auth

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc"
	"github.com/golang-jwt/jwt/v4"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/rand"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"
)

type OauthConfig struct {
	ClientID     string
	ClientSecret string
	TenantID     string
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
	dryRun       bool
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

func NewAzureClient(dryRun bool, clientID, clientSecret, tenantID string, log *logrus.Entry) *Azure {
	if dryRun {
		log.Infof("NOOP: Running in dry run mode")
		return &Azure{
			dryRun: dryRun,
			log:    log,
		}
	}

	provider, err := oidc.NewProvider(context.Background(), fmt.Sprintf("https://login.microsoftonline.com/%v/v2.0", tenantID))
	if err != nil {
		panic(err)
	}

	a := &Azure{
		clientID:     clientID,
		clientSecret: clientSecret,
		tenantID:     tenantID,
		provider:     provider,
		dryRun:       dryRun,
		log:          log,
	}

	a.setupOAuth2()
	return a
}

func (a *Azure) setupOAuth2() {
	redirectURL := "https://knorten.knada.io/oauth2/callback"
	if a.dryRun {
		redirectURL = "http://localhost:8080/oauth2/callback"
	}

	a.Config = oauth2.Config{
		ClientID:     a.clientID,
		ClientSecret: a.clientSecret,
		Endpoint:     a.provider.Endpoint(),
		RedirectURL:  redirectURL,
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
	if a.dryRun {
		fmt.Printf("NOOP: Would have checked if user %v exists in Azure AD\n", user)
		return nil
	}

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

	resBytes, err := io.ReadAll(res.Body)
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
	if a.dryRun {
		a.log.Infof("NOOP: Running in dry run mode")
		return fmt.Sprintf("d%v", rand.Intn(10000)+100000), nil
	}

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

	resBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	var identRes identResponse
	if err := json.Unmarshal(resBytes, &identRes); err != nil {
		return "", err
	}

	if identRes.Ident == "" {
		return "", fmt.Errorf("unable to get user ident for email %v", email)
	}

	return strings.ToLower(identRes.Ident), nil
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
	if a.dryRun {
		a.log.Infof("NOOP: Running in dry run mode")
		return "dummyID", nil
	}

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
		return "", errors.New("group not found by the mail")
	}
}

type CertificateList []*x509.Certificate

type KeyDiscovery struct {
	Keys []Key `json:"keys"`
}

type EncodedCertificate string

type Key struct {
	Kid string               `json:"kid"`
	X5c []EncodedCertificate `json:"x5c"`
}

// Map transform a KeyDiscovery object into a dictionary with "kid" as key
// and lists of decoded X509 certificates as values.
//
// Returns an error if any certificate does not decode.
func (k *KeyDiscovery) Map() (result map[string]CertificateList, err error) {
	result = make(map[string]CertificateList)

	for _, key := range k.Keys {
		certList := make(CertificateList, 0)
		for _, encodedCertificate := range key.X5c {
			certificate, err := encodedCertificate.Decode()
			if err != nil {
				return nil, err
			}
			certList = append(certList, certificate)
		}
		result[key.Kid] = certList
	}

	return
}

// Decode a base64 encoded certificate into a X509 structure.
func (c EncodedCertificate) Decode() (*x509.Certificate, error) {
	stream := strings.NewReader(string(c))
	decoder := base64.NewDecoder(base64.StdEncoding, stream)
	key, err := io.ReadAll(decoder)
	if err != nil {
		return nil, err
	}

	return x509.ParseCertificate(key)
}

func DiscoverURL(url string) (*KeyDiscovery, error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	return Discover(response.Body)
}

func Discover(reader io.Reader) (*KeyDiscovery, error) {
	document, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	keyDiscovery := &KeyDiscovery{}
	err = json.Unmarshal(document, keyDiscovery)

	return keyDiscovery, err
}

func JWTValidator(certificates map[string]CertificateList, audience string) jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		var certificateList CertificateList
		var kid string
		var ok bool

		if claims, ok := token.Claims.(*jwt.MapClaims); !ok {
			return nil, fmt.Errorf("unable to retrieve claims from token")
		} else {
			if valid := claims.VerifyAudience(audience, true); !valid {
				return nil, fmt.Errorf("the token is not valid for this application")
			}
		}

		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		if kid, ok = token.Header["kid"].(string); !ok {
			return nil, fmt.Errorf("field 'kid' is of invalid type %T, should be string", token.Header["kid"])
		}

		if certificateList, ok = certificates[kid]; !ok {
			return nil, fmt.Errorf("kid '%s' not found in certificate list", kid)
		}

		for _, certificate := range certificateList {
			return certificate.PublicKey, nil
		}

		return nil, fmt.Errorf("no certificate candidates for kid '%s'", kid)
	}
}
