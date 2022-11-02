package auth

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v4"
)

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
	key, err := ioutil.ReadAll(decoder)
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
	document, err := ioutil.ReadAll(reader)
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
