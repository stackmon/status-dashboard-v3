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
			groups:         []interface{}{"admin-group", "user-group"},
			requiredGroup:  "admin-group",
			expectedResult: true,
		},
		{
			name:           "Required group not present",
			groups:         []interface{}{"user-group", "other-group"},
			requiredGroup:  "admin-group",
			expectedResult: false,
		},
		{
			name:           "Empty groups array",
			groups:         []interface{}{},
			requiredGroup:  "admin-group",
			expectedResult: false,
		},
		{
			name:           "Single matching group",
			groups:         []interface{}{"admin-group"},
			requiredGroup:  "admin-group",
			expectedResult: true,
		},
		{
			name:           "Multiple groups with match",
			groups:         []interface{}{"group1", "group2", "admin-group", "group3"},
			requiredGroup:  "admin-group",
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
		// No groups claim
	}

	token := &jwt.Token{
		Claims: claims,
	}

	result := isAuthGroupInClaims(token, logger, "admin-group")
	assert.False(t, result)
}

func TestIsAuthGroupInClaims_InvalidGroupsType(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	claims := jwt.MapClaims{
		"sub":    "test-user",
		"groups": "not-an-array", // Invalid type
	}

	token := &jwt.Token{
		Claims: claims,
	}

	result := isAuthGroupInClaims(token, logger, "admin-group")
	assert.False(t, result)
}

func TestIsAuthGroupInClaims_GroupsWithNonStringElements(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	claims := jwt.MapClaims{
		"sub":    "test-user",
		"groups": []interface{}{123, "admin-group", true}, // Mixed types
	}

	token := &jwt.Token{
		Claims: claims,
	}

	result := isAuthGroupInClaims(token, logger, "admin-group")
	assert.True(t, result) // Should still find the string "admin-group"
}

func TestIsAuthGroupInClaims_InvalidClaimsType(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Use a different claims type that's not MapClaims
	type CustomClaims struct {
		jwt.RegisteredClaims
		Groups []string
	}

	token := &jwt.Token{
		Claims: CustomClaims{
			Groups: []string{"admin-group"},
		},
	}

	result := isAuthGroupInClaims(token, logger, "admin-group")
	assert.False(t, result) // Should fail because it's not MapClaims
}

// BenchmarkIsAuthGroupInClaims benchmarks the group checking function.
//
//nolint:intrange
func BenchmarkIsAuthGroupInClaims(b *testing.B) {
	logger, _ := zap.NewDevelopment()

	claims := jwt.MapClaims{
		"sub":    "test-user",
		"groups": []interface{}{"group1", "group2", "admin-group", "group3", "group4"},
	}

	token := &jwt.Token{
		Claims: claims,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = isAuthGroupInClaims(token, logger, "admin-group")
	}
}

// helper to set unexported field realmPublicKey on auth.Provider using reflect+unsafe
func setRealmPublicKey(prov *auth.Provider, key *rsa.PublicKey) {
	val := reflect.ValueOf(prov).Elem()
	field := val.FieldByName("realmPublicKey")
	ptrToField := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
	ptrToField.Set(reflect.ValueOf(key))
}

func TestParseToken_HMAC_Success(t *testing.T) {
	secret := "supersecret"
	// create token signed with HS256
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "123"})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	logger := zaptest.NewLogger(t)
	logger.Debug("HMAC success test - signed token", zap.String("token", signed))

	parsed, err := parseToken(signed, secret, nil, logger)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if !parsed.Valid {
		t.Fatalf("expected token to be valid")
	}
	logger.Debug("HMAC success test - token valid")
}

func TestParseToken_HMAC_WrongSecret(t *testing.T) {
	secret := "supersecret"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "123"})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	logger := zaptest.NewLogger(t)
	logger.Debug("HMAC wrong secret test - signed token", zap.String("token", signed))

	_, err = parseToken(signed, "wrongsecret", nil, logger)
	if err == nil {
		t.Fatalf("expected error when using wrong secret")
	}
	logger.Debug("HMAC wrong secret test - expected failure observed")
}

func TestParseToken_RSA_Success(t *testing.T) {
	// generate RSA key pair
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate rsa key: %v", err)
	}

	// sign token with private key
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"sub": "rsa-user"})
	signed, err := token.SignedString(priv)
	if err != nil {
		t.Fatalf("failed to sign rsa token: %v", err)
	}

	// provider with matching public key (set via helper)
	prov := &auth.Provider{}
	setRealmPublicKey(prov, &priv.PublicKey)

	logger := zaptest.NewLogger(t)
	logger.Debug("RSA success test - signed token", zap.String("token", signed))
	logger.Debug("RSA success test - public key set on provider")

	parsed, err := parseToken(signed, "", prov, logger)
	if err != nil {
		t.Fatalf("unexpected parse error for rsa token: %v", err)
	}
	if !parsed.Valid {
		t.Fatalf("expected rsa token to be valid")
	}
	logger.Debug("RSA success test - token valid")
}

func TestParseToken_RSA_WrongPublicKey(t *testing.T) {
	// generate two distinct RSA key pairs
	priv1, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate rsa key1: %v", err)
	}
	priv2, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate rsa key2: %v", err)
	}

	// sign with priv1 but provide pub2 to parser
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"sub": "rsa-user"})
	signed, err := token.SignedString(priv1)
	if err != nil {
		t.Fatalf("failed to sign rsa token: %v", err)
	}

	prov := &auth.Provider{}
	setRealmPublicKey(prov, &priv2.PublicKey)

	logger := zaptest.NewLogger(t)
	logger.Debug("RSA wrong pubkey test - signed token", zap.String("token", signed))
	logger.Debug("RSA wrong pubkey test - different public key set on provider")

	_, err = parseToken(signed, "", prov, logger)
	if err == nil {
		t.Fatalf("expected error when public key does not match signature")
	}
	logger.Debug("RSA wrong pubkey test - expected failure observed")
}

// performRequestWithAuth runs a small router with the provided middleware and an endpoint.
func performRequestWithAuth(mw gin.HandlerFunc, authHeader string) *httptest.ResponseRecorder {
	router := gin.New()
	router.Use(mw)
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestAuthenticationMW_HMAC_SuccessAndFailures(t *testing.T) {
	secret := "supersecret"
	// create token signed with HS256
	tkn := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "123"})
	signed, err := tkn.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	logger := zaptest.NewLogger(t)

	prov := &auth.Provider{}
	mw := AuthenticationMW(prov, logger, secret, "") // no group requirement for HMAC
	w := performRequestWithAuth(mw, "Bearer "+signed)
	assert.Equal(t, http.StatusOK, w.Code, "expected middleware to allow valid HMAC token")

	// failure: missing header
	w = performRequestWithAuth(mw, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code, "expected 401 when no Authorization header")

	// failure: wrong secret configured
	mwWrong := AuthenticationMW(prov, logger, "wrong-secret", "")
	w = performRequestWithAuth(mwWrong, "Bearer "+signed)
	assert.Equal(t, http.StatusUnauthorized, w.Code, "expected 401 when secret does not match")
}

func TestAuthenticationMW_RSA_WithAndWithoutGroup(t *testing.T) {
	// generate RSA key pair
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate rsa key: %v", err)
	}

	// success case: token signed with private key and contains required group "/admin-group"
	claimsWithGroup := jwt.MapClaims{
		"sub":    "rsa-user",
		"groups": []interface{}{"/admin-group"},
	}
	tokenWithGroup := jwt.NewWithClaims(jwt.SigningMethodRS256, claimsWithGroup)
	signedWithGroup, err := tokenWithGroup.SignedString(priv)
	if err != nil {
		t.Fatalf("failed to sign rsa token: %v", err)
	}

	prov := &auth.Provider{}
	// set unexported realmPublicKey so prov.GetPublicKey() returns immediately
	val := reflect.ValueOf(prov).Elem()
	field := val.FieldByName("realmPublicKey")
	ptrToField := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
	ptrToField.Set(reflect.ValueOf(&priv.PublicKey))

	logger := zaptest.NewLogger(t)

	mw := AuthenticationMW(prov, logger, "", "admin-group") // require "admin-group"
	w := performRequestWithAuth(mw, "Bearer "+signedWithGroup)
	assert.Equal(t, http.StatusOK, w.Code, "expected middleware to allow RSA token when group present")

	// failure case: token signed with same key but missing required group
	claimsWithoutGroup := jwt.MapClaims{
		"sub":    "rsa-user",
		"groups": []interface{}{"other-group"},
	}
	tokenWithoutGroup := jwt.NewWithClaims(jwt.SigningMethodRS256, claimsWithoutGroup)
	signedWithoutGroup, err := tokenWithoutGroup.SignedString(priv)
	if err != nil {
		t.Fatalf("failed to sign rsa token: %v", err)
	}

	w = performRequestWithAuth(mw, "Bearer "+signedWithoutGroup)
	assert.Equal(t, http.StatusUnauthorized, w.Code, "expected 401 when RSA token lacks required group")
}
