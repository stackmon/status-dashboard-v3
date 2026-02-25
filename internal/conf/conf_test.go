package conf

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRBACConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    RBACConfig
		expectErr bool
		errSubstr string
	}{
		{
			name:      "Disabled RBAC: no groups required",
			config:    RBACConfig{Disabled: true},
			expectErr: false,
		},
		{
			name: "Enabled with all groups configured",
			config: RBACConfig{
				Disabled:  false,
				Creators:  "sd_creators",
				Operators: "sd_operators",
				Admins:    "sd_admins",
			},
			expectErr: false,
		},
		{
			name: "Enabled but missing Creators",
			config: RBACConfig{
				Disabled:  false,
				Operators: "sd_operators",
				Admins:    "sd_admins",
			},
			expectErr: true,
			errSubstr: "SD_RBAC_GROUP_CREATORS",
		},
		{
			name: "Enabled but missing Operators",
			config: RBACConfig{
				Disabled: false,
				Creators: "sd_creators",
				Admins:   "sd_admins",
			},
			expectErr: true,
			errSubstr: "SD_RBAC_GROUP_OPERATORS",
		},
		{
			name: "Enabled but missing Admins",
			config: RBACConfig{
				Disabled:  false,
				Creators:  "sd_creators",
				Operators: "sd_operators",
			},
			expectErr: true,
			errSubstr: "SD_RBAC_GROUP_ADMINS",
		},
		{
			name: "Enabled but all groups missing",
			config: RBACConfig{
				Disabled: false,
			},
			expectErr: true,
			errSubstr: "SD_RBAC_GROUP_CREATORS",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errSubstr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfig_Validate_PropagatesRBACError(t *testing.T) {
	cfg := &Config{
		Port: "8000",
		RBAC: RBACConfig{
			Disabled: false,
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RBAC is enabled")
}
