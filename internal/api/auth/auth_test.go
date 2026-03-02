package auth

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/oauth2"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

// newTestProvider creates a Provider with a mock Keycloak server for testing.
func newTestProvider(t *testing.T, kcHandler http.HandlerFunc) (*Provider, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(kcHandler)

	kc := &Keycloak{
		httpClient:   ts.Client(),
		clientID:     "test-client",
		clientSecret: "test-secret",
		tokenURL:     ts.URL + "/token",
		logoutURL:    ts.URL + "/logout",
		jwksURL:      ts.URL + "/certs",
	}

	conf := &oauth2.Config{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Endpoint:     oauth2.Endpoint{TokenURL: ts.URL + "/token"},
	}

	prov := &Provider{
		WebURL:  "http://localhost:9000",
		kc:      kc,
		conf:    conf,
		storage: newInternalStorage(),
	}

	return prov, ts
}

func TestProvider_ClientID(t *testing.T) {
	t.Run("with config", func(t *testing.T) {
		p := &Provider{conf: &oauth2.Config{ClientID: "my-client"}}
		assert.Equal(t, "my-client", p.ClientID())
	})

	t.Run("nil config", func(t *testing.T) {
		p := &Provider{}
		assert.Empty(t, p.ClientID())
	})
}

func TestProvider_PutGetToken(t *testing.T) {
	prov := &Provider{storage: newInternalStorage()}

	prov.PutToken("challenge", TokenRepr{AccessToken: "at", RefreshToken: "rt"})
	token, ok := prov.GetToken("challenge")
	require.True(t, ok)
	assert.Equal(t, "at", token.AccessToken)
	assert.Equal(t, "rt", token.RefreshToken)

	// GetToken is consume-once
	_, ok = prov.GetToken("challenge")
	assert.False(t, ok, "second GetToken should return false (token consumed)")
}

func TestGetLoginPageHandler_MissingState(t *testing.T) {
	logger := zaptest.NewLogger(t)
	prov := &Provider{conf: &oauth2.Config{}}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/auth/login", nil)

	handler := GetLoginPageHandler(prov, logger)
	handler(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetLoginPageHandler_WithState(t *testing.T) {
	logger := zaptest.NewLogger(t)
	prov := &Provider{
		conf: &oauth2.Config{
			ClientID: "test",
			Endpoint: oauth2.Endpoint{AuthURL: "http://kc.test/auth"},
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/auth/login?state=abc", nil)

	handler := GetLoginPageHandler(prov, logger)
	handler(c)
	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "http://kc.test/auth")
}

func TestPostTokenHandler_MissingBody(t *testing.T) {
	logger := zaptest.NewLogger(t)
	prov := &Provider{storage: newInternalStorage()}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/token", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	handler := PostTokenHandler(prov, logger)
	handler(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPostTokenHandler_InvalidCodeVerifier(t *testing.T) {
	logger := zaptest.NewLogger(t)
	prov := &Provider{storage: newInternalStorage()}

	body, _ := json.Marshal(CodeVerifierReq{CodeVerifier: "wrong-verifier"})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/token", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler := PostTokenHandler(prov, logger)
	handler(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPostTokenHandler_ValidCodeVerifier(t *testing.T) {
	logger := zaptest.NewLogger(t)
	prov := &Provider{storage: newInternalStorage()}

	// Compute code_challenge = SHA256(code_verifier)
	verifier := "my-code-verifier-12345"
	h := sha256.New()
	h.Write([]byte(verifier))
	challenge := hex.EncodeToString(h.Sum(nil))

	prov.PutToken(challenge, TokenRepr{AccessToken: "at-ok", RefreshToken: "rt-ok"})

	body, _ := json.Marshal(CodeVerifierReq{CodeVerifier: verifier})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/token", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler := PostTokenHandler(prov, logger)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp TokenRepr
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "at-ok", resp.AccessToken)
	assert.Equal(t, "rt-ok", resp.RefreshToken)
}

func TestPutLogoutHandler_MissingBody(t *testing.T) {
	logger := zaptest.NewLogger(t)
	prov := &Provider{storage: newInternalStorage()}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/auth/logout", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	handler := PutLogoutHandler(prov, logger)
	handler(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPutLogoutHandler_KeycloakSuccess(t *testing.T) {
	logger := zaptest.NewLogger(t)

	prov, ts := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/logout" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
	})
	defer ts.Close()

	body, _ := json.Marshal(PutLogoutReq{RefreshToken: "valid-rt"})

	router := gin.New()
	router.PUT("/auth/logout", PutLogoutHandler(prov, logger))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/auth/logout", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestPutLogoutHandler_KeycloakError(t *testing.T) {
	logger := zaptest.NewLogger(t)

	prov, ts := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/logout" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(KeycloakExternalError{
				ErrorOrig:        "invalid_grant",
				ErrorDescription: "Token is not active",
			})
			return
		}
	})
	defer ts.Close()

	body, _ := json.Marshal(PutLogoutReq{RefreshToken: "expired-rt"})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/auth/logout", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler := PutLogoutHandler(prov, logger)
	handler(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPostRefreshHandler_MissingBody(t *testing.T) {
	logger := zaptest.NewLogger(t)
	prov := &Provider{storage: newInternalStorage()}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	handler := PostRefreshHandler(prov, logger)
	handler(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPostRefreshHandler_Success(t *testing.T) {
	logger := zaptest.NewLogger(t)

	prov, ts := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(TokenRepr{AccessToken: "new-at", RefreshToken: "new-rt"})
			return
		}
	})
	defer ts.Close()

	body, _ := json.Marshal(RefreshTokenReq{RefreshToken: "old-rt"})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler := PostRefreshHandler(prov, logger)
	handler(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp TokenRepr
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "new-at", resp.AccessToken)
	assert.Equal(t, "new-rt", resp.RefreshToken)
}

func TestPostRefreshHandler_KeycloakError(t *testing.T) {
	logger := zaptest.NewLogger(t)

	prov, ts := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(KeycloakExternalError{
				ErrorOrig:        "invalid_grant",
				ErrorDescription: "Session not active",
			})
			return
		}
	})
	defer ts.Close()

	body, _ := json.Marshal(RefreshTokenReq{RefreshToken: "expired-rt"})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler := PostRefreshHandler(prov, logger)
	handler(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestKeycloak_FetchPublicKey_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Minimal JWKS response with a valid RSA key
		jwks := `{
			"keys": [{
				"kty": "RSA",
				"alg": "RS256",
				"use": "sig",
				"kid": "test-kid",
				"n": "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
				"e": "AQAB"
			}]
		}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, jwks)
	}))
	defer ts.Close()

	kc := &Keycloak{
		httpClient: ts.Client(),
		jwksURL:    ts.URL + "/certs",
	}

	pubKey, err := kc.fetchPublicKey()
	require.NoError(t, err)
	assert.NotNil(t, pubKey)
	assert.Equal(t, 65537, pubKey.E)
}

func TestKeycloak_FetchPublicKey_NoRSAKey(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"keys": []}`)
	}))
	defer ts.Close()

	kc := &Keycloak{
		httpClient: ts.Client(),
		jwksURL:    ts.URL + "/certs",
	}

	_, err := kc.fetchPublicKey()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no RSA public key found")
}

func TestKeycloak_FetchPublicKey_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "not json")
	}))
	defer ts.Close()

	kc := &Keycloak{
		httpClient: ts.Client(),
		jwksURL:    ts.URL + "/certs",
	}

	_, err := kc.fetchPublicKey()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error decoding JWK set")
}

func TestKeycloakExternalError_Error(t *testing.T) {
	err := KeycloakExternalError{
		ErrorOrig:        "invalid_grant",
		ErrorDescription: "Token is not active",
	}
	assert.Equal(t, "Token is not active", err.Error())
}

func TestNewKeycloak_Endpoints(t *testing.T) {
	kc := NewKeycloak("http://kc.test", "myrealm", "client-id", "client-secret")

	assert.Equal(t, "http://kc.test/realms/myrealm", kc.issuer)
	assert.Contains(t, kc.authURL, "/protocol/openid-connect/auth")
	assert.Contains(t, kc.tokenURL, "/protocol/openid-connect/token")
	assert.Contains(t, kc.jwksURL, "/protocol/openid-connect/certs")
	assert.Contains(t, kc.logoutURL, "/protocol/openid-connect/logout")

	ep := kc.Endpoint()
	assert.Equal(t, kc.authURL, ep.AuthURL)
	assert.Equal(t, kc.tokenURL, ep.TokenURL)
}

func TestProvider_GetPublicKey_Caches(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		jwks := `{
			"keys": [{
				"kty": "RSA", "alg": "RS256", "use": "sig", "kid": "k1",
				"n": "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
				"e": "AQAB"
			}]
		}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, jwks)
	}))
	defer ts.Close()

	prov := &Provider{
		kc: &Keycloak{httpClient: ts.Client(), jwksURL: ts.URL + "/certs"},
	}

	key1, err := prov.GetPublicKey()
	require.NoError(t, err)

	key2, err := prov.GetPublicKey()
	require.NoError(t, err)

	assert.Equal(t, key1, key2)
	assert.Equal(t, 1, callCount, "fetchPublicKey should be called only once (cached)")
}

func TestProvider_GetPublicKey_RetryOnFailure(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		// First 2 attempts fail, third succeeds
		if callCount < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		jwks := `{
			"keys": [{
				"kty": "RSA", "alg": "RS256", "use": "sig", "kid": "k1",
				"n": "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw",
				"e": "AQAB"
			}]
		}`
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, jwks)
	}))
	defer ts.Close()

	prov := &Provider{
		kc: &Keycloak{httpClient: ts.Client(), jwksURL: ts.URL + "/certs"},
	}

	key, err := prov.GetPublicKey()
	require.NoError(t, err)
	assert.NotNil(t, key)
	assert.Equal(t, 3, callCount, "should have retried 3 times")
}

func TestProvider_GetPublicKey_AllRetriesFail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	prov := &Provider{
		kc: &Keycloak{httpClient: ts.Client(), jwksURL: ts.URL + "/certs"},
	}

	_, err := prov.GetPublicKey()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch public key after")
}
