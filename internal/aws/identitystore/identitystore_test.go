package identitystore

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ssosync/internal/interfaces"
	"ssosync/internal/mocks"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	"github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/aws/smithy-go"
	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCreateGroup(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	displayName := "Test Group"

	expectedOutput := &identitystore.CreateGroupOutput{
		GroupId:         aws.String("group-123"),
		IdentityStoreId: aws.String(identityStoreID),
	}

	mockClient.EXPECT().CreateGroup(ctx, &identitystore.CreateGroupInput{
		IdentityStoreId: &identityStoreID,
		DisplayName:     &displayName,
	}).Return(expectedOutput, nil)

	result, err := CreateGroup(ctx, mockClient, &identityStoreID, &displayName)

	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, result)
}

func TestCreateGroupMembership(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	groupID := "group-123"
	userID := "user-456"

	expectedOutput := &identitystore.CreateGroupMembershipOutput{
		MembershipId:    aws.String("membership-789"),
		IdentityStoreId: aws.String(identityStoreID),
	}

	mockClient.EXPECT().CreateGroupMembership(ctx, &identitystore.CreateGroupMembershipInput{
		IdentityStoreId: &identityStoreID,
		GroupId:         &groupID,
		MemberId: &types.MemberIdMemberUserId{
			Value: userID,
		},
	}).Return(expectedOutput, nil)

	result, err := CreateGroupMembership(ctx, mockClient, &identityStoreID, &groupID, &userID)

	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, result)
}

func TestDeleteGroup(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	groupID := "group-123"

	expectedOutput := &identitystore.DeleteGroupOutput{}

	mockClient.EXPECT().DeleteGroup(ctx, &identitystore.DeleteGroupInput{
		IdentityStoreId: &identityStoreID,
		GroupId:         &groupID,
	}).Return(expectedOutput, nil)

	result, err := DeleteGroup(ctx, mockClient, &identityStoreID, &groupID)

	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, result)
}

func TestDeleteUser(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	userID := "user-456"

	expectedOutput := &identitystore.DeleteUserOutput{}

	mockClient.EXPECT().DeleteUser(ctx, &identitystore.DeleteUserInput{
		IdentityStoreId: &identityStoreID,
		UserId:          &userID,
	}).Return(expectedOutput, nil)

	result, err := DeleteUser(ctx, mockClient, &identityStoreID, &userID)

	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, result)
}

func TestIsMemberInGroups_True(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	groupIDs := []string{"group-123", "group-456"}
	userID := "user-789"

	expectedOutput := &identitystore.IsMemberInGroupsOutput{
		Results: []types.GroupMembershipExistenceResult{
			{
				GroupId:          aws.String("group-123"),
				MembershipExists: true,
			},
			{
				GroupId:          aws.String("group-456"),
				MembershipExists: false,
			},
		},
	}

	mockClient.EXPECT().IsMemberInGroups(ctx, &identitystore.IsMemberInGroupsInput{
		IdentityStoreId: &identityStoreID,
		MemberId: &types.MemberIdMemberUserId{
			Value: userID,
		},
		GroupIds: groupIDs,
	}).Return(expectedOutput, nil)

	result, err := IsMemberInGroups(ctx, mockClient, &identityStoreID, groupIDs, &userID)

	assert.NoError(t, err)
	assert.True(t, *result)
}

func TestIsMemberInGroups_False(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	groupIDs := []string{"group-123", "group-456"}
	userID := "user-789"

	expectedOutput := &identitystore.IsMemberInGroupsOutput{
		Results: []types.GroupMembershipExistenceResult{
			{
				GroupId:          aws.String("group-123"),
				MembershipExists: false,
			},
			{
				GroupId:          aws.String("group-456"),
				MembershipExists: false,
			},
		},
	}

	mockClient.EXPECT().IsMemberInGroups(ctx, &identitystore.IsMemberInGroupsInput{
		IdentityStoreId: &identityStoreID,
		MemberId: &types.MemberIdMemberUserId{
			Value: userID,
		},
		GroupIds: groupIDs,
	}).Return(expectedOutput, nil)

	result, err := IsMemberInGroups(ctx, mockClient, &identityStoreID, groupIDs, &userID)

	assert.NoError(t, err)
	assert.False(t, *result)
}

// Paginator tests are commented out due to interface compatibility issues
// between the generated mocks and the actual AWS SDK paginator types.
// These would be better tested with integration tests or by using the actual
// AWS SDK paginators in a test environment.

func TestIsMemberInGroups_Error(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	groupIDs := []string{"group-123"}
	userID := "user-789"

	mockClient.EXPECT().IsMemberInGroups(ctx, &identitystore.IsMemberInGroupsInput{
		IdentityStoreId: &identityStoreID,
		MemberId: &types.MemberIdMemberUserId{
			Value: userID,
		},
		GroupIds: groupIDs,
	}).Return(nil, assert.AnError)

	result, err := IsMemberInGroups(ctx, mockClient, &identityStoreID, groupIDs, &userID)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestGetGroupMembershipId(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	groupID := "group-123"
	userID := "user-456"

	expectedOutput := &identitystore.GetGroupMembershipIdOutput{
		MembershipId:    aws.String("membership-789"),
		IdentityStoreId: aws.String(identityStoreID),
	}

	mockClient.EXPECT().GetGroupMembershipId(ctx, &identitystore.GetGroupMembershipIdInput{
		IdentityStoreId: &identityStoreID,
		GroupId:         &groupID,
		MemberId:        &types.MemberIdMemberUserId{Value: userID},
	}).Return(expectedOutput, nil)

	result, err := GetGroupMembershipId(ctx, mockClient, &identityStoreID, &groupID, &userID)

	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, result)
}

func TestGetGroupMembershipId_Error(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	groupID := "group-123"
	userID := "user-456"

	mockClient.EXPECT().GetGroupMembershipId(ctx, &identitystore.GetGroupMembershipIdInput{
		IdentityStoreId: &identityStoreID,
		GroupId:         &groupID,
		MemberId:        &types.MemberIdMemberUserId{Value: userID},
	}).Return(nil, assert.AnError)

	result, err := GetGroupMembershipId(ctx, mockClient, &identityStoreID, &groupID, &userID)

	assert.ErrorIs(t, err, assert.AnError)
	assert.Nil(t, result)
}

func TestDeleteGroupMembership(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	membershipID := "membership-789"

	expectedOutput := &identitystore.DeleteGroupMembershipOutput{}

	mockClient.EXPECT().DeleteGroupMembership(ctx, &identitystore.DeleteGroupMembershipInput{
		IdentityStoreId: &identityStoreID,
		MembershipId:    &membershipID,
	}).Return(expectedOutput, nil)

	result, err := DeleteGroupMembership(ctx, mockClient, &identityStoreID, &membershipID)

	assert.NoError(t, err)
	assert.Equal(t, expectedOutput, result)
}

func TestDeleteGroupMembership_Error(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	membershipID := "membership-789"

	mockClient.EXPECT().DeleteGroupMembership(ctx, &identitystore.DeleteGroupMembershipInput{
		IdentityStoreId: &identityStoreID,
		MembershipId:    &membershipID,
	}).Return(nil, assert.AnError)

	result, err := DeleteGroupMembership(ctx, mockClient, &identityStoreID, &membershipID)

	assert.ErrorIs(t, err, assert.AnError)
	assert.Nil(t, result)
}

// groupConverter converts SDK groups to interfaces.Group, dropping groups with
// a nil DisplayName so the nil-filter branch is exercised.
func groupConverter(g types.Group) *interfaces.Group {
	if g.DisplayName == nil {
		return nil
	}
	return &interfaces.Group{ID: aws.ToString(g.GroupId), DisplayName: aws.ToString(g.DisplayName)}
}

func TestListGroups_MultiPage(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"

	mockClient.EXPECT().ListGroups(ctx, &identitystore.ListGroupsInput{IdentityStoreId: &identityStoreID}, mock.Anything).
		Return(&identitystore.ListGroupsOutput{
			Groups: []types.Group{
				{GroupId: aws.String("g1"), DisplayName: aws.String("one")},
				{GroupId: aws.String("g2")}, // filtered out
			},
			NextToken: aws.String("page2"),
		}, nil).Once()
	mockClient.EXPECT().ListGroups(ctx, &identitystore.ListGroupsInput{IdentityStoreId: &identityStoreID, NextToken: aws.String("page2")}, mock.Anything).
		Return(&identitystore.ListGroupsOutput{
			Groups: []types.Group{{GroupId: aws.String("g3"), DisplayName: aws.String("three")}},
		}, nil).Once()

	groups, err := ListGroups(ctx, mockClient, &identityStoreID, groupConverter)

	assert.NoError(t, err)
	assert.Len(t, groups, 2)
	assert.Equal(t, "g1", groups[0].ID)
	assert.Equal(t, "g3", groups[1].ID)
}

func TestListGroups_Error(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"

	mockClient.EXPECT().ListGroups(ctx, mock.Anything, mock.Anything).Return(nil, assert.AnError)

	groups, err := ListGroups(ctx, mockClient, &identityStoreID, groupConverter)

	assert.Error(t, err)
	assert.Nil(t, groups)
}

func TestListGroupsPager(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"

	mockClient.EXPECT().ListGroups(ctx, mock.Anything, mock.Anything).
		Return(&identitystore.ListGroupsOutput{
			Groups: []types.Group{
				{GroupId: aws.String("g1"), DisplayName: aws.String("one")},
				{GroupId: aws.String("g2")}, // filtered out
			},
		}, nil).Once()

	paginator := identitystore.NewListGroupsPaginator(mockClient, &identitystore.ListGroupsInput{IdentityStoreId: &identityStoreID})
	groups, err := ListGroupsPager(ctx, paginator, groupConverter)

	assert.NoError(t, err)
	assert.Len(t, groups, 1)
	assert.Equal(t, "g1", groups[0].ID)
}

func TestListGroupsPager_Error(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"

	mockClient.EXPECT().ListGroups(ctx, mock.Anything, mock.Anything).Return(nil, assert.AnError)

	paginator := identitystore.NewListGroupsPaginator(mockClient, &identitystore.ListGroupsInput{IdentityStoreId: &identityStoreID})
	groups, err := ListGroupsPager(ctx, paginator, groupConverter)

	assert.Error(t, err)
	assert.Nil(t, groups)
}

func TestListUsersPager(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"

	mockClient.EXPECT().ListUsers(ctx, mock.Anything, mock.Anything).
		Return(&identitystore.ListUsersOutput{
			Users:     []types.User{{UserId: aws.String("u1"), UserName: aws.String("a")}},
			NextToken: aws.String("page2"),
		}, nil).Once()
	mockClient.EXPECT().ListUsers(ctx, mock.Anything, mock.Anything).
		Return(&identitystore.ListUsersOutput{
			Users: []types.User{{UserId: aws.String("u2")}}, // filtered out
		}, nil).Once()

	converter := func(u types.User) *interfaces.User {
		if u.UserName == nil {
			return nil
		}
		return &interfaces.User{ID: aws.ToString(u.UserId), Username: aws.ToString(u.UserName)}
	}

	paginator := identitystore.NewListUsersPaginator(mockClient, &identitystore.ListUsersInput{IdentityStoreId: &identityStoreID})
	users, err := ListUsersPager(ctx, paginator, converter)

	assert.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, "u1", users[0].ID)
}

func TestListUsersPager_Error(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"

	mockClient.EXPECT().ListUsers(ctx, mock.Anything, mock.Anything).Return(nil, assert.AnError)

	paginator := identitystore.NewListUsersPaginator(mockClient, &identitystore.ListUsersInput{IdentityStoreId: &identityStoreID})
	users, err := ListUsersPager(ctx, paginator, func(types.User) *interfaces.User { return nil })

	assert.Error(t, err)
	assert.Nil(t, users)
}

func TestListGroupMembershipsPager(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	groupID := "group-123"

	mockClient.EXPECT().ListGroupMemberships(ctx, mock.Anything, mock.Anything).
		Return(&identitystore.ListGroupMembershipsOutput{
			GroupMemberships: []types.GroupMembership{
				{MemberId: &types.MemberIdMemberUserId{Value: "u1"}},
				{MemberId: nil}, // filtered out
			},
		}, nil).Once()

	converter := func(m types.GroupMembership) *interfaces.User {
		member, ok := m.MemberId.(*types.MemberIdMemberUserId)
		if !ok {
			return nil
		}
		return &interfaces.User{ID: member.Value}
	}

	paginator := identitystore.NewListGroupMembershipsPaginator(mockClient, &identitystore.ListGroupMembershipsInput{
		IdentityStoreId: &identityStoreID,
		GroupId:         &groupID,
	})
	users, err := ListGroupMembershipsPager(ctx, paginator, converter)

	assert.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, "u1", users[0].ID)
}

func TestListGroupMembershipsPager_Error(t *testing.T) {
	mockClient := mocks.NewMockIdentityStoreAPI(t)
	ctx := context.Background()
	identityStoreID := "d-123456789"
	groupID := "group-123"

	mockClient.EXPECT().ListGroupMemberships(ctx, mock.Anything, mock.Anything).Return(nil, assert.AnError)

	paginator := identitystore.NewListGroupMembershipsPaginator(mockClient, &identitystore.ListGroupMembershipsInput{
		IdentityStoreId: &identityStoreID,
		GroupId:         &groupID,
	})
	users, err := ListGroupMembershipsPager(ctx, paginator, func(types.GroupMembership) *interfaces.User { return nil })

	assert.Error(t, err)
	assert.Nil(t, users)
}

// #429: ListGroupMemberships ThrottlingException fails ~5% of groups-mode runs (Identity Store client uses default 3 retries, no pacing)

// throttleHTTP is a fake Identity Store endpoint: the first `throttles`
// requests get a ThrottlingException, the rest an empty ListGroupMemberships
// page. It records when each request arrived.
type throttleHTTP struct {
	mu        sync.Mutex
	throttles int
	times     []time.Time
}

func (f *throttleHTTP) Do(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.times = append(f.times, time.Now())
	status, body := http.StatusOK, `{"GroupMemberships":[]}`
	if len(f.times) <= f.throttles {
		status, body = http.StatusBadRequest, `{"__type":"ThrottlingException","message":"Rate exceeded"}`
	}
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/x-amz-json-1.1"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

func (f *throttleHTTP) requests() []time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]time.Time(nil), f.times...)
}

func throttleTestConfig(f *throttleHTTP) aws.Config {
	return aws.Config{Region: "us-east-1", Credentials: aws.AnonymousCredentials{}, HTTPClient: f}
}

func listTestMemberships(c *identitystore.Client) error {
	_, err := c.ListGroupMemberships(context.Background(), &identitystore.ListGroupMembershipsInput{
		IdentityStoreId: aws.String("d-1"),
		GroupId:         aws.String("g"),
	})
	return err
}

// fastStandard swaps in a standard retryer with 1 ms backoff, so the attempt
// tests are not slowed by adaptive mode's own token waits after a throttle.
// NewClient's MaxAttempts default still applies (finalizeRetryMaxAttempts).
func fastStandard(o *identitystore.Options) {
	o.Retryer = retry.AddWithMaxBackoffDelay(retry.NewStandard(), time.Millisecond)
}

// assertPaced checks that request i+1 came at least i/RequestsPerSecond after
// the first (1 ms tolerance), so the run never exceeded RequestsPerSecond. A
// single gap can be a little under 1/RequestsPerSecond when the previous
// wait overslept: rate.Limiter schedules tokens on its own timeline.
func assertPaced(t *testing.T, times []time.Time) {
	t.Helper()
	interval := time.Second / RequestsPerSecond
	for i := 1; i < len(times); i++ {
		assert.GreaterOrEqual(t, times[i].Sub(times[0]), time.Duration(i)*interval-time.Millisecond,
			"request %d after the first", i+1)
	}
}

func TestNewClient_Defaults(t *testing.T) {
	c := NewClient(throttleTestConfig(&throttleHTTP{}), "eu-west-1")
	assert.Equal(t, aws.RetryModeAdaptive, c.Options().RetryMode)
	assert.Equal(t, 10, c.Options().RetryMaxAttempts)
	assert.Equal(t, "eu-west-1", c.Options().Region)
}

// isolateAWSConfig points the shared config and credentials files at path, so
// the developer's own ~/.aws files cannot change the result.
func isolateAWSConfig(t *testing.T, path string) {
	t.Helper()
	t.Setenv("AWS_CONFIG_FILE", path)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_RETRY_MODE", "")
	t.Setenv("AWS_MAX_ATTEMPTS", "")
}

func loadTestConfig(t *testing.T) aws.Config {
	t.Helper()
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(aws.AnonymousCredentials{}))
	require.NoError(t, err)
	return cfg
}

func TestNewClient_EnvWins(t *testing.T) {
	isolateAWSConfig(t, filepath.Join(t.TempDir(), "missing"))
	t.Setenv("AWS_RETRY_MODE", "standard")
	t.Setenv("AWS_MAX_ATTEMPTS", "2")
	cfg := loadTestConfig(t)
	f := &throttleHTTP{throttles: 100}
	cfg.HTTPClient = f

	c := NewClient(cfg, "us-east-1", func(o *identitystore.Options) {
		o.Retryer = retry.AddWithMaxBackoffDelay(o.Retryer, time.Millisecond)
	})
	assert.Equal(t, aws.RetryModeStandard, c.Options().RetryMode)
	assert.Equal(t, 2, c.Options().RetryMaxAttempts)

	var maxErr *retry.MaxAttemptsError
	require.True(t, errors.As(listTestMemberships(c), &maxErr))
	assert.Equal(t, 2, maxErr.Attempt)
	assert.Len(t, f.requests(), 2)
}

func TestNewClient_SharedConfigWins(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(path, []byte("[default]\nretry_mode = standard\nmax_attempts = 4\n"), 0o600))
	isolateAWSConfig(t, path)

	c := NewClient(loadTestConfig(t), "us-east-1")
	assert.Equal(t, aws.RetryModeStandard, c.Options().RetryMode)
	assert.Equal(t, 4, c.Options().RetryMaxAttempts)
}

// Nine throttles then success: 10 requests, and every attempt is paced,
// retries included.
func TestNewClient_ThrottledNineTimesSucceeds(t *testing.T) {
	f := &throttleHTTP{throttles: 9}
	c := NewClient(throttleTestConfig(f), "us-east-1", fastStandard)

	require.NoError(t, listTestMemberships(c))
	times := f.requests()
	assert.Len(t, times, 10)
	assertPaced(t, times)
}

func TestNewClient_ThrottledTenTimesFails(t *testing.T) {
	f := &throttleHTTP{throttles: 10}
	c := NewClient(throttleTestConfig(f), "us-east-1", fastStandard)

	err := listTestMemberships(c)
	var maxErr *retry.MaxAttemptsError
	require.True(t, errors.As(err, &maxErr))
	assert.Equal(t, 10, maxErr.Attempt)
	var apiErr smithy.APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, "ThrottlingException", apiErr.ErrorCode())
	assert.Len(t, f.requests(), 10)
}

// The limiter is per client, so separate calls are paced too.
func TestWithRateLimit_PacesCalls(t *testing.T) {
	f := &throttleHTTP{}
	c := NewClient(throttleTestConfig(f), "us-east-1")
	for i := 0; i < 5; i++ {
		require.NoError(t, listTestMemberships(c))
	}
	times := f.requests()
	assert.Len(t, times, 5)
	assertPaced(t, times)
}

// With the resolved adaptive retryer (backoff cut to 1 ms), the gap after a
// throttle is far longer than the backoff or the 18/s interval: only adaptive
// mode's client-side rate limiter explains it.
func TestNewClient_AdaptiveBacksOffAfterThrottle(t *testing.T) {
	f := &throttleHTTP{throttles: 1}
	c := NewClient(throttleTestConfig(f), "us-east-1", func(o *identitystore.Options) {
		o.Retryer = retry.AddWithMaxBackoffDelay(o.Retryer, time.Millisecond)
	})

	require.NoError(t, listTestMemberships(c))
	times := f.requests()
	require.Len(t, times, 2)
	assert.GreaterOrEqual(t, times[1].Sub(times[0]), 300*time.Millisecond)
}

func TestWithRateLimit_LogsThrottle(t *testing.T) {
	hook := logtest.NewGlobal()
	defer hook.Reset()
	f := &throttleHTTP{throttles: 1}
	c := NewClient(throttleTestConfig(f), "us-east-1", fastStandard)

	require.NoError(t, listTestMemberships(c))
	var throttled []*log.Entry
	for _, e := range hook.AllEntries() {
		if e.Message == "Identity Store request throttled" {
			throttled = append(throttled, e)
		}
	}
	require.Len(t, throttled, 1)
	assert.Equal(t, log.WarnLevel, throttled[0].Level)
	assert.Equal(t, "ListGroupMemberships", throttled[0].Data["operation"])
}
