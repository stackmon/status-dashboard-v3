package api

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
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

// BenchmarkIsAuthGroupInClaims benchmarks the group checking function
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
