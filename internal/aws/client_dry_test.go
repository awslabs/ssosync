package aws

import (
	"errors"
	"testing"

	"github.com/awslabs/ssosync/internal/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubSCIMClient is a minimal stub of the SCIM Client interface for dry-client tests.
type stubSCIMClient struct {
	findUserResult *interfaces.User
	findUserErr    error
}

func (s *stubSCIMClient) CreateUser(u *interfaces.User) (*interfaces.User, error) {
	return u, nil
}
func (s *stubSCIMClient) FindGroupByDisplayName(name string) (*interfaces.Group, error) {
	return nil, ErrGroupNotFound
}
func (s *stubSCIMClient) FindUserByEmail(email string) (*interfaces.User, error) {
	return s.findUserResult, s.findUserErr
}
func (s *stubSCIMClient) UpdateUser(u *interfaces.User) (*interfaces.User, error) {
	return u, nil
}

func newTestDryClient(t *testing.T, stub *stubSCIMClient) *dryClient {
	t.Helper()
	if stub == nil {
		stub = &stubSCIMClient{findUserErr: ErrUserNotFound}
	}
	return &dryClient{
		c:            stub,
		virtualUsers: make(map[string]interfaces.User),
	}
}

func TestDryClient_CreateUser_PopulatesVirtualID(t *testing.T) {
	dc := newTestDryClient(t, nil)
	u := NewUser("Alice", "Smith", "alice@example.com", true)

	result, err := dc.CreateUser(u)

	require.NoError(t, err)
	assert.Equal(t, virtualUserID("alice@example.com"), result.ID)
	assert.True(t, isVirtualID(result.ID))
	assert.Regexp(t, awsIDRegex, result.ID)
	stored := dc.virtualUsers["alice@example.com"]
	assert.Equal(t, result.ID, stored.ID)
}

func TestDryClient_CreateUser_Deterministic(t *testing.T) {
	dc := newTestDryClient(t, nil)
	u1 := NewUser("Alice", "Smith", "alice@example.com", true)
	u2 := NewUser("Alice", "Smith", "alice@example.com", true)

	r1, _ := dc.CreateUser(u1)
	r2, _ := dc.CreateUser(u2)

	assert.Equal(t, r1.ID, r2.ID, "same email must produce the same virtual ID")
}

func TestDryClient_FindUserByEmail_ReturnsVirtualUserWithID(t *testing.T) {
	dc := newTestDryClient(t, &stubSCIMClient{findUserErr: ErrUserNotFound})
	u := NewUser("Bob", "Jones", "bob@example.com", true)
	_, err := dc.CreateUser(u)
	require.NoError(t, err)

	found, err := dc.FindUserByEmail("bob@example.com")

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, virtualUserID("bob@example.com"), found.ID)
	assert.True(t, isVirtualID(found.ID))
}

func TestDryClient_UpdateUser_PreservesExistingID(t *testing.T) {
	dc := newTestDryClient(t, nil)
	u := UpdateUser("real-aws-id-1234", "Carol", "White", "carol@example.com", false)

	result, err := dc.UpdateUser(u)

	require.NoError(t, err)
	assert.Equal(t, "real-aws-id-1234", result.ID, "real ID must not be replaced")
}

func TestDryClient_UpdateUser_PopulatesVirtualIDWhenEmpty(t *testing.T) {
	dc := newTestDryClient(t, nil)
	u := NewUser("Dan", "Brown", "dan@example.com", true)

	result, err := dc.UpdateUser(u)

	require.NoError(t, err)
	assert.Equal(t, virtualUserID("dan@example.com"), result.ID)
}

func TestDryClient_FindUserByEmail_ForwardsRealError(t *testing.T) {
	dc := newTestDryClient(t, &stubSCIMClient{findUserErr: errors.New("network error")})

	result, err := dc.FindUserByEmail("err@example.com")

	assert.Nil(t, result)
	assert.EqualError(t, err, "network error")
}
