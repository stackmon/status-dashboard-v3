package rbac

import "strings"

type Role int

const (
	NoRole   Role = 0
	Creator  Role = 10
	Operator Role = 30
	Admin    Role = 50
)

type Service struct {
	creatorsGroup  string
	operatorsGroup string
	adminsGroup    string
}

func New(creatorsGroup, operatorsGroup, adminsGroup string) *Service {
	return &Service{
		creatorsGroup:  creatorsGroup,
		operatorsGroup: operatorsGroup,
		adminsGroup:    adminsGroup,
	}
}

// HasAuthorizedGroup checks if the user belongs to any configured RBAC group.
// Group names are normalized by trimming leading "/" prefix.
// The non-empty guards are defense in depth — conf.Validate() ensures groups
// are set when RBAC is enabled, but this package must remain safe standalone.
func (s *Service) HasAuthorizedGroup(userGroups []string) bool {
	for _, group := range userGroups {
		normalizedGroup := strings.TrimPrefix(group, "/")

		if normalizedGroup == s.adminsGroup && s.adminsGroup != "" {
			return true
		}
		if normalizedGroup == s.operatorsGroup && s.operatorsGroup != "" {
			return true
		}
		if normalizedGroup == s.creatorsGroup && s.creatorsGroup != "" {
			return true
		}
	}
	return false
}

// Resolve determines the highest RBAC role from the user's group membership.
// The non-empty guards are defense in depth — see HasAuthorizedGroup.
func (s *Service) Resolve(userGroups []string) Role {
	currentRole := NoRole

	for _, group := range userGroups {
		normalizedGroup := strings.TrimPrefix(group, "/")

		if normalizedGroup == s.adminsGroup && s.adminsGroup != "" {
			return Admin
		}

		if normalizedGroup == s.operatorsGroup && s.operatorsGroup != "" {
			if Operator > currentRole {
				currentRole = Operator
			}
			continue
		}

		if normalizedGroup == s.creatorsGroup && s.creatorsGroup != "" {
			if Creator > currentRole {
				currentRole = Creator
			}
			continue
		}
	}

	return currentRole
}

func (r Role) IsAdmin() bool    { return r >= Admin }
func (r Role) CanApprove() bool { return r >= Operator }
func (r Role) CanCreate() bool  { return r >= Creator }
