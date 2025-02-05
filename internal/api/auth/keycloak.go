package auth

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

const defaultTimeout = 10

type Keycloak struct {
	httpClient *http.Client

	clientID     string
	clientSecret string

	issuer        string
	authURL       string
	tokenURL      string
	deviceAuthURL string
	userInfoURL   string
	jwksURL       string
	logoutURL     string
}

func NewKeycloak(url, realm, clientID, clientSecret string) *Keycloak {
	// you can get all endpoint from endpoint realms/{realm}/.well-known/openid-configuration
	issuer := fmt.Sprintf("%s/realms/%s", url, realm)
	authURL := fmt.Sprintf("%s/protocol/openid-connect/auth", issuer)
	tokenURL := fmt.Sprintf("%s/protocol/openid-connect/token", issuer)
	deviceAuthURL := fmt.Sprintf("%s/protocol/openid-connect/auth/device", issuer)
	userInfoURL := fmt.Sprintf("%s/protocol/openid-connect/userinfo", issuer)
	jwksURL := fmt.Sprintf("%s/protocol/openid-connect/certs", issuer)
	logoutURL := fmt.Sprintf("%s/protocol/openid-connect/logout", issuer)

	httpClient := &http.Client{
		Timeout: time.Second * defaultTimeout,
	}

	return &Keycloak{
		httpClient: httpClient,

		clientID:     clientID,
		clientSecret: clientSecret,

		issuer:        issuer,
		authURL:       authURL,
		tokenURL:      tokenURL,
		deviceAuthURL: deviceAuthURL,
		userInfoURL:   userInfoURL,
		jwksURL:       jwksURL,
		logoutURL:     logoutURL,
	}
}

func (kc *Keycloak) Endpoint() oauth2.Endpoint {
	return oauth2.Endpoint{AuthURL: kc.authURL, DeviceAuthURL: kc.deviceAuthURL, TokenURL: kc.tokenURL}
}

type JWKSet struct {
	Keys []JWK `json:"keys"`
}

type JWK struct {
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func (kc *Keycloak) fetchPublicKey() (*rsa.PublicKey, error) {
	req, err := http.NewRequest(http.MethodGet, kc.jwksURL, nil) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	resp, err := kc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error fetching JWK set: %w", err)
	}
	defer resp.Body.Close()

	var jwkSet JWKSet
	err = json.NewDecoder(resp.Body).Decode(&jwkSet)
	if err != nil {
		return nil, fmt.Errorf("error decoding JWK set: %w", err)
	}

	var rsaPublicKey JWK
	for _, key := range jwkSet.Keys {
		if key.Kty == "RSA" && key.Use == "sig" {
			rsaPublicKey = key
		}
	}
	nBytes, err := base64.RawURLEncoding.DecodeString(rsaPublicKey.N)
	if err != nil {
		return nil, fmt.Errorf("error decoding N: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(rsaPublicKey.E)
	if err != nil {
		return nil, fmt.Errorf("error decoding E: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := int(new(big.Int).SetBytes(eBytes).Int64())

	pubKey := &rsa.PublicKey{
		N: n,
		E: e,
	}
	return pubKey, nil
}

type KeycloakExternalError struct {
	ErrorOrig        string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func (e KeycloakExternalError) Error() string {
	return e.ErrorDescription
}

func (kc *Keycloak) revokeToken(refreshToken string) error {
	data := url.Values{}
	data.Set("client_id", kc.clientID)
	data.Set("client_secret", kc.clientSecret)
	data.Set("refresh_token", refreshToken)

	req, err := http.NewRequest(http.MethodPost, kc.logoutURL, strings.NewReader(data.Encode())) //nolint:noctx
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := kc.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		var errResp KeycloakExternalError
		if err = json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return err
		}

		return errResp
	}

	return nil
}
