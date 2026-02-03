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
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/stackmon/otc-status-dashboard/internal/api/auth"
)

func TestIsAuthGroupInClaims(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	tests := []struct {
		name           string
		groups         []interface{}
		requiredGroup  string
		expectedResult bool
	}{
		{
			name:           "Valid group present",
			groups:         []interface{}{"sd-admins", "sd-operators"},
			requiredGroup:  "sd-admins",
			expectedResult: true,
		},
		{
			name:           "Required group not present",
			groups:         []interface{}{"sd-operators", "other-group"},
			requiredGroup:  "sd-admins",
			expectedResult: false,
		},
		{
			name:           "Empty groups array",
			groups:         []interface{}{},
			requiredGroup:  "sd-admins",
			expectedResult: false,
		},
		{
			name:           "Single matching group",
			groups:         []interface{}{"sd-admins"},
			requiredGroup:  "sd-admins",
			expectedResult: true,
		},
		{
			name:           "Multiple groups with match",
			groups:         []interface{}{"group1", "group2", "sd-admins", "group3"},
			requiredGroup:  "sd-admins",
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := jwt.MapClaims{
				"sub":    "test-user",
				"groups": tt.groups,
			}

			token := &jwt.Token{
				Claims: claims,
			}

			result := isAuthGroupInClaims(token, logger, tt.requiredGroup)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestIsAuthGroupInClaims_MissingGroupsClaim(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	claims := jwt.MapClaims{
		"sub": "test-user",
	}

	token := &jwt.Token{
		Claims: claims,
	}

	result := isAuthGroupInClaims(token, logger, "sd-admins")
	assert.False(t, result)
}

func TestIsAuthGroupInClaims_InvalidGroupsType(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	claims := jwt.MapClaims{
		"sub":    "test-user",
		"groups": "not-an-array",
	}

	token := &jwt.Token{
		Claims: claims,
	}

	result := isAuthGroupInClaims(token, logger, "sd-admins")
	assert.False(t, result)
}

func TestIsAuthGroupInClaims_GroupsWithNonStringElements(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	claims := jwt.MapClaims{
		"sub":    "test-user",
		"groups": []interface{}{123, "sd-admins", true},
	}

	token := &jwt.Token{
		Claims: claims,
	}

	result := isAuthGroupInClaims(token, logger, "sd-admins")
	assert.True(t, result)
}

func TestIsAuthGroupInClaims_InvalidClaimsType(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	type CustomClaims struct {
		jwt.RegisteredClaims
		Groups []string
	}

	token := &jwt.Token{
		Claims: CustomClaims{
			Groups: []string{"sd-admins"},
		},
	}

	result := isAuthGroupInClaims(token, logger, "sd-admins")
	assert.False(t, result)
}

func BenchmarkIsAuthGroupInClaims(b *testing.B) {
	logger, _ := zap.NewDevelopment()

	claims := jwt.MapClaims{
		"sub":    "test-user",
		"groups": []interface{}{"group1", "group2", "sd-admins", "group3", "group4"},
	}

	token := &jwt.Token{
		Claims: claims,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = isAuthGroupInClaims(token, logger, "sd-admins")
	}
}

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
	mw := AuthenticationMW(prov, logger, secret, "")
	w := performRequestWithAuth(mw, "Bearer "+signed)
	assert.Equal(t, http.StatusOK, w.Code, "expected middleware to allow valid HMAC token")

	w = performRequestWithAuth(mw, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code, "expected 401 when no Authorization header")

	mwWrong := AuthenticationMW(prov, logger, "wrong-secret", "")
	w = performRequestWithAuth(mwWrong, "Bearer "+signed)
	assert.Equal(t, http.StatusUnauthorized, w.Code, "expected 401 when secret does not match")
}

func TestAuthenticationMW_RSA_WithAndWithoutGroup(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "failed to generate rsa key")

	claimsWithGroup := jwt.MapClaims{
		"sub":    "rsa-user",
		"groups": []interface{}{"/sd-admins"},
	}
	tokenWithGroup := jwt.NewWithClaims(jwt.SigningMethodRS256, claimsWithGroup)
	signedWithGroup, err := tokenWithGroup.SignedString(priv)
	require.NoError(t, err, "failed to sign rsa token")

	prov := &auth.Provider{}
	val := reflect.ValueOf(prov).Elem()
	field := val.FieldByName("realmPublicKey")
	ptrToField := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
	ptrToField.Set(reflect.ValueOf(&priv.PublicKey))

	logger := zaptest.NewLogger(t)

	mw := AuthenticationMW(prov, logger, "", "sd-admins")
	w := performRequestWithAuth(mw, "Bearer "+signedWithGroup)
	assert.Equal(t, http.StatusOK, w.Code, "expected middleware to allow RSA token when group present")

	claimsWithoutGroup := jwt.MapClaims{
		"sub":    "rsa-user",
		"groups": []interface{}{"other-group"},
	}
	tokenWithoutGroup := jwt.NewWithClaims(jwt.SigningMethodRS256, claimsWithoutGroup)
	signedWithoutGroup, err := tokenWithoutGroup.SignedString(priv)
	require.NoError(t, err, "failed to sign rsa token")

	w = performRequestWithAuth(mw, "Bearer "+signedWithoutGroup)
	assert.Equal(t, http.StatusUnauthorized, w.Code, "expected 401 when RSA token lacks required group")
}
