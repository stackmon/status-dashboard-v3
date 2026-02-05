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

func (s *Service) Resolve(userGroups []string) Role {
	currentRole := NoRole

	for _, group := range userGroups {
		normalizedGroup := strings.TrimPrefix(group, "/")

		if normalizedGroup == s.adminsGroup {
			return Admin
		}

		if normalizedGroup == s.operatorsGroup {
			if Operator > currentRole {
				currentRole = Operator
			}
			continue
		}

		if normalizedGroup == s.creatorsGroup {
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
