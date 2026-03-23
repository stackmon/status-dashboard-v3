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
	admins    map[string]struct{}
	operators map[string]struct{}
	creators  map[string]struct{}
}

func parseGroups(input string) map[string]struct{} {
	m := make(map[string]struct{})
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		part = strings.TrimPrefix(part, "/")
		if part != "" {
			m[part] = struct{}{}
		}
	}
	return m
}

func New(creatorsGroup, operatorsGroup, adminsGroup string) *Service {
	return &Service{
		creators:  parseGroups(creatorsGroup),
		operators: parseGroups(operatorsGroup),
		admins:    parseGroups(adminsGroup),
	}
}

func (s *Service) roleForGroup(group string) Role {
	if _, ok := s.admins[group]; ok {
		return Admin
	}
	if _, ok := s.operators[group]; ok {
		return Operator
	}
	if _, ok := s.creators[group]; ok {
		return Creator
	}
	return NoRole
}

func normalizeGroup(group string) string {
	return strings.TrimPrefix(group, "/")
}

func (s *Service) HasAuthorizedGroup(userGroups []string) bool {
	for _, group := range userGroups {
		g := normalizeGroup(group)
		if s.roleForGroup(g) != NoRole {
			return true
		}
	}
	return false
}

func (s *Service) ResolveRole(userGroups []string) Role {
	currentRole := NoRole
	for _, group := range userGroups {
		g := normalizeGroup(group)
		r := s.roleForGroup(g)
		if r == Admin {
			return Admin
		}
		if r > currentRole {
			currentRole = r
		}
	}
	return currentRole
}

func (r Role) IsAdmin() bool    { return r >= Admin }
func (r Role) CanApprove() bool { return r >= Operator }
func (r Role) CanCreate() bool  { return r >= Creator }
