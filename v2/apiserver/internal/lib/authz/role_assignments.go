package authz

import (
	"encoding/json"

	"github.com/brigadecore/brigade/v2/apiserver/internal/meta"
)

// RoleAssignmentKind represents the canonical RoleAssignment kind string
const RoleAssignmentKind = "RoleAssignment"

// RoleAssignment represents the assignment of a Role to a principal such as a
// User or ServiceAccount.
type RoleAssignment struct {
	// Role assigns a Role to the specified principal.
	Role Role `json:"role" bson:"role"`
	// Principal specifies the principal to whom the Role is assigned.
	Principal PrincipalReference `json:"principal" bson:"principal"`
	// Scope qualifies the scope of the Role. The value is opaque and has meaning
	// only in relation to a specific Role.
	Scope string `json:"scope,omitempty" bson:"scope,omitempty"`
}

// MarshalJSON amends RoleAssignment instances with type metadata.
func (r RoleAssignment) MarshalJSON() ([]byte, error) {
	type Alias RoleAssignment
	return json.Marshal(
		struct {
			meta.TypeMeta `json:",inline"`
			Alias         `json:",inline"`
		}{
			TypeMeta: meta.TypeMeta{
				APIVersion: meta.APIVersion,
				Kind:       RoleAssignmentKind,
			},
			Alias: (Alias)(r),
		},
	)
}

// Matches determines if this RoleAssignment matches the role and scope
// arguments.
func (r RoleAssignment) Matches(role Role, scope string) bool {
	return r.Role == role &&
		(r.Scope == scope || r.Scope == RoleScopeGlobal)
}