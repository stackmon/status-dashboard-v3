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
func (s *Service) HasAuthorizedGroup(userGroups []string) bool {
	for _, group := range userGroups {
		normalizedGroup := strings.TrimPrefix(group, "/")

		if s.adminsGroup != "" && normalizedGroup == s.adminsGroup {
			return true
		}
		if s.operatorsGroup != "" && normalizedGroup == s.operatorsGroup {
			return true
		}
		if s.creatorsGroup != "" && normalizedGroup == s.creatorsGroup {
			return true
		}
	}
	return false
}

// Resolve determines the highest RBAC role from the user's group membership.
func (s *Service) Resolve(userGroups []string) Role {
	currentRole := NoRole

	for _, group := range userGroups {
		normalizedGroup := strings.TrimPrefix(group, "/")

		if s.adminsGroup != "" && normalizedGroup == s.adminsGroup {
			return Admin
		}

		if s.operatorsGroup != "" && normalizedGroup == s.operatorsGroup {
			if Operator > currentRole {
				currentRole = Operator
			}
			continue
		}

		if s.creatorsGroup != "" && normalizedGroup == s.creatorsGroup {
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
