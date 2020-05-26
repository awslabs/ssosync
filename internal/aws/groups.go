package aws

// NewGroup creates an object representing a group with the given name
func NewGroup(groupName string) *Group{
	return &Group{
		Schemas:     []string{"urn:ietf:params:scim:schemas:core:2.0:Group"},
		DisplayName: groupName,
	}
}
