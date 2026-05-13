package internal

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	aws_sdk "github.com/aws/aws-sdk-go-v2/aws"
	aws_identitystore "github.com/aws/aws-sdk-go-v2/service/identitystore"
	idstypes "github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/awslabs/ssosync/internal/aws"
	"github.com/awslabs/ssosync/internal/config"
	"github.com/awslabs/ssosync/internal/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	admin "google.golang.org/api/admin/directory/v1"
)

// stubGoogleClient is a minimal Google client stub for dry-run sync tests.
type stubGoogleClient struct {
	groups       []*admin.Group
	groupMembers map[string][]*admin.Member
	usersByQuery map[string][]*admin.User
}

func (s *stubGoogleClient) GetDeletedUsers() ([]*admin.User, error) { return nil, nil }

func (s *stubGoogleClient) GetGroups(_ string) ([]*admin.Group, error) { return s.groups, nil }

func (s *stubGoogleClient) GetGroupMembers(g *admin.Group) ([]*admin.Member, error) {
	return s.groupMembers[g.Id], nil
}

func (s *stubGoogleClient) GetUsers(query, _ string) ([]*admin.User, error) {
	return s.usersByQuery[query], nil
}

// TestSyncGroupsUsers_DryRun_NewUserNotInAWS_DoesNotCrash reproduces the crash
// from https://github.com/awslabs/ssosync/issues/281: when DRY_RUN=true, a
// Google user that does not yet exist in AWS Identity Store must not cause a
// ValidationException from an empty memberId.userId.
func TestSyncGroupsUsers_DryRun_NewUserNotInAWS_DoesNotCrash(t *testing.T) {
	// SCIM server: alice does not exist in AWS yet — every lookup returns 0 results.
	// dryClient.CreateUser doesn't make HTTP calls, so only GET /Users hits the server.
	scimServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/scim+json")
		fmt.Fprintln(w, `{"totalResults":0,"Resources":[]}`)
	}))
	defer scimServer.Close()

	awsClient, err := aws.NewDryClient(scimServer.Client(), &aws.Config{
		Endpoint: scimServer.URL,
		Token:    "test-token",
	})
	require.NoError(t, err)

	mockIDS := mocks.NewMockIdentityStoreAPI(t)
	dryIDS := aws.NewDryIdentityStore(mockIDS)

	const identityStoreID = "d-test"
	const engGroupID = "12345678-1234-1234-1234-123456789012"

	// Engineering exists in AWS; alice does not.
	mockIDS.EXPECT().ListGroups(mock.Anything, mock.Anything, mock.Anything).
		Return(&aws_identitystore.ListGroupsOutput{
			Groups: []idstypes.Group{
				{GroupId: aws_sdk.String(engGroupID), DisplayName: aws_sdk.String("Engineering")},
			},
		}, nil).Once()
	mockIDS.EXPECT().ListUsers(mock.Anything, mock.Anything, mock.Anything).
		Return(&aws_identitystore.ListUsersOutput{}, nil).Once()
	mockIDS.EXPECT().ListGroupMemberships(mock.Anything, mock.Anything, mock.Anything).
		Return(&aws_identitystore.ListGroupMembershipsOutput{}, nil).Once()
	// IsMemberInGroups and CreateGroupMembership short-circuit in DryIdentityStore
	// when the user ID is virtual — the real AWS client is never called.

	gClient := &stubGoogleClient{
		groups: []*admin.Group{
			{Id: "google-eng-id", Name: "Engineering", Email: "eng@example.com"},
		},
		groupMembers: map[string][]*admin.Member{
			"google-eng-id": {
				{Email: "alice@example.com", Type: "USER", Status: "ACTIVE"},
			},
		},
		usersByQuery: map[string][]*admin.User{
			"email=alice@example.com": {
				{
					Id:           "google-alice-id",
					PrimaryEmail: "alice@example.com",
					Name:         &admin.UserName{GivenName: "Alice", FamilyName: "Smith"},
				},
			},
		},
	}

	cfg := &config.Config{
		IdentityStoreID: identityStoreID,
	}

	syncer := New(cfg, awsClient, gClient, dryIDS)
	require.NoError(t, syncer.SyncGroupsUsers("Engineering", ""))
}
