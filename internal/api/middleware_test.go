package api

import (
	"crypto/rand"
	"crypto/rsa"
	"reflect"
	"testing"
	"unsafe"

	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/stackmon/otc-status-dashboard/internal/api/auth"
	"github.com/stackmon/otc-status-dashboard/internal/api/rbac"
	v2 "github.com/stackmon/otc-status-dashboard/internal/api/v2"
)


func setRealmPublicKey(prov *auth.Provider, key *rsa.PublicKey) {
	val := reflect.ValueOf(prov).Elem()
	field := val.FieldByName("realmPublicKey")
	ptrToField := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
	ptrToField.Set(reflect.ValueOf(key))
}

func TestParseToken_HMAC_Success(t *testing.T) {
	secret := "supersecret"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "123"})
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err, "failed to sign token")

	logger := zaptest.NewLogger(t)

	parsed, err := parseToken(signed, secret, nil, logger)
	require.NoError(t, err, "unexpected parse error")
	assert.True(t, parsed.Valid, "expected token to be valid")
}

func TestParseToken_HMAC_WrongSecret(t *testing.T) {
	secret := "supersecret"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "123"})
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err, "failed to sign token")

	logger := zaptest.NewLogger(t)

	_, err = parseToken(signed, "wrongsecret", nil, logger)
	require.Error(t, err, "expected error when using wrong secret")
}

func TestParseToken_RSA_Success(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "failed to generate rsa key")

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"sub": "rsa-user"})
	signed, err := token.SignedString(priv)
	require.NoError(t, err, "failed to sign rsa token")

	prov := &auth.Provider{}
	setRealmPublicKey(prov, &priv.PublicKey)

	logger := zaptest.NewLogger(t)

	parsed, err := parseToken(signed, "", prov, logger)
	require.NoError(t, err, "unexpected parse error for rsa token")
	assert.True(t, parsed.Valid, "expected rsa token to be valid")
}

func TestParseToken_RSA_WrongPublicKey(t *testing.T) {
	priv1, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "failed to generate rsa key1")
	priv2, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "failed to generate rsa key2")

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"sub": "rsa-user"})
	signed, err := token.SignedString(priv1)
	require.NoError(t, err, "failed to sign rsa token")

	prov := &auth.Provider{}
	setRealmPublicKey(prov, &priv2.PublicKey)

	logger := zaptest.NewLogger(t)

	_, err = parseToken(signed, "", prov, logger)
	require.Error(t, err, "expected error when public key does not match signature")
}

func performRequestWithAuth(mw gin.HandlerFunc, authHeader string) *httptest.ResponseRecorder {
	router := gin.New()
	router.Use(mw)
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestAuthenticationMW_HMAC_SuccessAndFailures(t *testing.T) {
	secret := "supersecret"
	tkn := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "123"})
	signed, err := tkn.SignedString([]byte(secret))
	require.NoError(t, err, "failed to sign token")

	logger := zaptest.NewLogger(t)

	prov := &auth.Provider{}
	mw := AuthenticationMW(prov, logger, secret)
	w := performRequestWithAuth(mw, "Bearer "+signed)
	assert.Equal(t, http.StatusOK, w.Code, "expected middleware to allow valid HMAC token")

	w = performRequestWithAuth(mw, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code, "expected 401 when no Authorization header")

	mwWrong := AuthenticationMW(prov, logger, "wrong-secret")
	w = performRequestWithAuth(mwWrong, "Bearer "+signed)
	assert.Equal(t, http.StatusUnauthorized, w.Code, "expected 401 when secret does not match")
}

func TestAuthenticationMW_RSA_ValidToken(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "failed to generate rsa key")

	claims := jwt.MapClaims{
		"sub":    "rsa-user",
		"groups": []interface{}{"/sd-admins"},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(priv)
	require.NoError(t, err, "failed to sign rsa token")

	prov := &auth.Provider{}
	val := reflect.ValueOf(prov).Elem()
	field := val.FieldByName("realmPublicKey")
	ptrToField := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
	ptrToField.Set(reflect.ValueOf(&priv.PublicKey))

	logger := zaptest.NewLogger(t)

	mw := AuthenticationMW(prov, logger, "")
	w := performRequestWithAuth(mw, "Bearer "+signed)
	assert.Equal(t, http.StatusOK, w.Code, "expected middleware to allow valid RSA token")
}

func TestRBACMiddleware_ValidGroups(t *testing.T) {
	logger := zaptest.NewLogger(t)
	rbacSvc := rbac.New("sd_creators", "sd_operators", "sd_admins")

	tests := []struct {
		name           string
		groups         []string
		expectedStatus int
		expectedRole   rbac.Role
	}{
		{
			name:           "Creator group is allowed",
			groups:         []string{"sd_creators"},
			expectedStatus: http.StatusOK,
			expectedRole:   rbac.Creator,
		},
		{
			name:           "Operator group is allowed",
			groups:         []string{"sd_operators"},
			expectedStatus: http.StatusOK,
			expectedRole:   rbac.Operator,
		},
		{
			name:           "Admin group is allowed",
			groups:         []string{"sd_admins"},
			expectedStatus: http.StatusOK,
			expectedRole:   rbac.Admin,
		},
		{
			name:           "Group with leading slash is normalized",
			groups:         []string{"/sd_creators"},
			expectedStatus: http.StatusOK,
			expectedRole:   rbac.Creator,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(v2.UserIDGroupsContextKey, tt.groups)
				c.Next()
			})
			router.Use(RBACAuthorizationMW(rbacSvc, logger))
			router.GET("/test", func(c *gin.Context) {
				role, _ := c.Get(roleContextKey)
				assert.Equal(t, tt.expectedRole, role)
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestRBACMiddleware_InvalidGroups(t *testing.T) {
	logger := zaptest.NewLogger(t)
	rbacSvc := rbac.New("sd_creators", "sd_operators", "sd_admins")

	tests := []struct {
		name           string
		groups         []string
		setGroups      bool
		expectedStatus int
	}{
		{
			name:           "Missing groups returns 401",
			setGroups:      false,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Empty groups array returns 401",
			groups:         []string{},
			setGroups:      true,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Unrecognized groups returns 401",
			groups:         []string{"random_group", "other_group"},
			setGroups:      true,
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				if tt.setGroups {
					c.Set(v2.UserIDGroupsContextKey, tt.groups)
				}
				c.Next()
			})
			router.Use(RBACAuthorizationMW(rbacSvc, logger))
			router.GET("/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestRBACMiddleware_NoClaims(t *testing.T) {
	logger := zaptest.NewLogger(t)
	rbacSvc := rbac.New("sd_creators", "sd_operators", "sd_admins")

	router := gin.New()
	router.Use(RBACAuthorizationMW(rbacSvc, logger))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRBACMiddleware_ExtractsUserID(t *testing.T) {
	logger := zaptest.NewLogger(t)
	rbacSvc := rbac.New("sd_creators", "sd_operators", "sd_admins")

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(v2.UsernameContextKey, "user-12345")
		c.Set(v2.UserIDGroupsContextKey, []string{"sd_creators"})
		c.Next()
	})
	router.Use(RBACAuthorizationMW(rbacSvc, logger))
	router.GET("/test", func(c *gin.Context) {
		userID, exists := c.Get(v2.UsernameContextKey)
		assert.True(t, exists)
		assert.Equal(t, "user-12345", userID)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
