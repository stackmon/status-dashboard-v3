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
			got := svc.Resolve(tt.groups)
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
