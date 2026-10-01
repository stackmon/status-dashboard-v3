package rbac

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestService_Resolve(t *testing.T) {
	svc := New("sd_creators", "sd_operators", "sd_admins")

	tests := []struct {
		name     string
		groups   []string
		expected Role
	}{
		{
			name:     "Empty groups list returns NoRole",
			groups:   []string{},
			expected: NoRole,
		},
		{
			name:     "Unrecognized group returns NoRole",
			groups:   []string{"some_random_group"},
			expected: NoRole,
		},
		{
			name:     "Creator group returns Creator role",
			groups:   []string{"sd_creators"},
			expected: Creator,
		},
		{
			name:     "Operator group returns Operator role",
			groups:   []string{"sd_operators"},
			expected: Operator,
		},
		{
			name:     "Admin group returns Admin role",
			groups:   []string{"sd_admins"},
			expected: Admin,
		},
		{
			name:     "Multiple roles: Operator supersedes Creator",
			groups:   []string{"sd_creators", "sd_operators"},
			expected: Operator,
		},
		{
			name:     "Multiple roles: Admin supersedes Operator",
			groups:   []string{"sd_operators", "sd_admins"},
			expected: Admin,
		},
		{
			name:     "Multiple roles: Admin supersedes all",
			groups:   []string{"sd_creators", "sd_operators", "sd_admins"},
			expected: Admin,
		},
		{
			name:     "Group normalization: handles leading slash for Creator",
			groups:   []string{"/sd_creators"},
			expected: Creator,
		},
		{
			name:     "Group normalization: handles leading slash for Admin",
			groups:   []string{"/sd_admins"},
			expected: Admin,
		},
		{
			name:     "Mixed normalized and raw groups",
			groups:   []string{"/sd_creators", "sd_operators"},
			expected: Operator,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.ResolveRole(tt.groups)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestRole_Permissions(t *testing.T) {
	tests := []struct {
		name       string
		role       Role
		canCreate  bool
		canApprove bool
		isAdmin    bool
	}{
		{
			name:       "NoRole has no permissions",
			role:       NoRole,
			canCreate:  false,
			canApprove: false,
			isAdmin:    false,
		},
		{
			name:       "Creator can create but not approve",
			role:       Creator,
			canCreate:  true,
			canApprove: false,
			isAdmin:    false,
		},
		{
			name:       "Operator can create and approve",
			role:       Operator,
			canCreate:  true,
			canApprove: true,
			isAdmin:    false,
		},
		{
			name:       "Admin has all permissions",
			role:       Admin,
			canCreate:  true,
			canApprove: true,
			isAdmin:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.canCreate, tt.role.CanCreate(), "CanCreate()")
			assert.Equal(t, tt.canApprove, tt.role.CanApprove(), "CanApprove()")
			assert.Equal(t, tt.isAdmin, tt.role.IsAdmin(), "IsAdmin()")
		})
	}
}

func TestService_HasAnyConfiguredGroup(t *testing.T) {
	svc := New("sd_creators", "sd_operators", "sd_admins")

	tests := []struct {
		name     string
		groups   []string
		expected bool
	}{
		{
			name:     "Empty groups list returns false",
			groups:   []string{},
			expected: false,
		},
		{
			name:     "Unrecognized group returns false",
			groups:   []string{"some_random_group"},
			expected: false,
		},
		{
			name:     "Creator group returns true",
			groups:   []string{"sd_creators"},
			expected: true,
		},
		{
			name:     "Operator group returns true",
			groups:   []string{"sd_operators"},
			expected: true,
		},
		{
			name:     "Admin group returns true",
			groups:   []string{"sd_admins"},
			expected: true,
		},
		{
			name:     "Group normalization: handles leading slash",
			groups:   []string{"/sd_creators"},
			expected: true,
		},
		{
			name:     "Mixed recognized and unrecognized groups",
			groups:   []string{"random", "other", "sd_operators"},
			expected: true,
		},
		{
			name:     "Only unrecognized groups",
			groups:   []string{"random", "other", "unknown"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.HasAuthorizedGroup(tt.groups)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestService_HasAnyConfiguredGroup_EmptyConfig(t *testing.T) {
	svc := New("", "", "")

	tests := []struct {
		name     string
		groups   []string
		expected bool
	}{
		{
			name:     "No groups configured, empty list returns false",
			groups:   []string{},
			expected: false,
		},
		{
			name:     "No groups configured, any group returns false",
			groups:   []string{"sd_creators", "sd_admins"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.HasAuthorizedGroup(tt.groups)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestParseGroups(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]struct{}
	}{
		{
			name:     "empty string returns empty map",
			input:    "",
			expected: map[string]struct{}{},
		},
		{
			name:     "single group",
			input:    "sd_admins",
			expected: map[string]struct{}{"sd_admins": {}},
		},
		{
			name:     "comma-separated groups (Vault real case)",
			input:    "sd-admins,status-dashboard",
			expected: map[string]struct{}{"sd-admins": {}, "status-dashboard": {}},
		},
		{
			name:     "spaces around commas are trimmed",
			input:    "sd_admins , status-dashboard",
			expected: map[string]struct{}{"sd_admins": {}, "status-dashboard": {}},
		},
		{
			name:     "leading slash in configured group is normalized",
			input:    "/sd_admins,/status-dashboard",
			expected: map[string]struct{}{"sd_admins": {}, "status-dashboard": {}},
		},
		{
			name:     "empty entries from double commas are ignored",
			input:    "sd_admins,,status-dashboard",
			expected: map[string]struct{}{"sd_admins": {}, "status-dashboard": {}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseGroups(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestService_CommaSeparatedConfig reproduces the real Vault scenario:
// SD_RBAC_GROUPS_ADMINS="sd-admins,status-dashboard"
// where Keycloak sends groups with a leading "/" prefix.
func TestService_CommaSeparatedConfig(t *testing.T) {
	// Mirrors Vault value: rbacgroupadmins = "sd-admins,status-dashboard"
	svc := New("sd_creators", "sd_operators", "sd-admins,status-dashboard")

	// Real token claims from preprod Keycloak (truncated for brevity)
	keycloakGroups := []string{
		"/argocd-admin",
		"/backstage",
		"/gitea-admin",
		"/gitea-users",
		"/grafana-admin",
		"/status-dashboard",
		"offline_access",
		"uma_authorization",
		"default-roles-eco",
	}

	t.Run("user with /status-dashboard is authorized", func(t *testing.T) {
		assert.True(t, svc.HasAuthorizedGroup(keycloakGroups))
	})

	t.Run("user with /status-dashboard resolves to Admin", func(t *testing.T) {
		assert.Equal(t, Admin, svc.ResolveRole(keycloakGroups))
	})

	t.Run("user with /sd-admins also resolves to Admin", func(t *testing.T) {
		assert.Equal(t, Admin, svc.ResolveRole([]string{"/sd-admins", "other-group"}))
	})

	t.Run("user without any matching group is denied", func(t *testing.T) {
		assert.False(t, svc.HasAuthorizedGroup([]string{"/argocd-admin", "offline_access"}))
	})
}

func TestService_Resolve_EmptyConfig(t *testing.T) {
	svc := New("", "", "")

	tests := []struct {
		name     string
		groups   []string
		expected Role
	}{
		{
			name:     "No groups configured, empty list returns NoRole",
			groups:   []string{},
			expected: NoRole,
		},
		{
			name:     "No groups configured, known names still return NoRole",
			groups:   []string{"sd_creators", "sd_operators", "sd_admins"},
			expected: NoRole,
		},
		{
			name:     "No groups configured, slash-prefixed returns NoRole",
			groups:   []string{"/"},
			expected: NoRole,
		},
		{
			name:     "No groups configured, empty string group returns NoRole",
			groups:   []string{""},
			expected: NoRole,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.ResolveRole(tt.groups)
			assert.Equal(t, tt.expected, got)
		})
	}
}
