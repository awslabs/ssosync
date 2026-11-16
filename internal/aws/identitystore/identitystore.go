package identitystore

import (
	"context"

	"ssosync/internal/interfaces"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	"github.com/aws/aws-sdk-go-v2/service/identitystore/types"
	"github.com/aws/smithy-go/middleware"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

// #429: the Identity Store APIs are limited to 20 TPS per API per Identity
// Store instance (https://docs.aws.amazon.com/singlesignon/latest/userguide/limits.html).
const (
	// MaxAttempts is the default number of attempts per Identity Store request
	// (SDK default 3). AWS_MAX_ATTEMPTS or shared-config max_attempts win.
	MaxAttempts = 10
	// RequestsPerSecond caps every Identity Store attempt, retries included.
	// It is below the 20 TPS limit to leave headroom for other callers.
	RequestsPerSecond = 18
)

// NewClient returns the Identity Store client ssosync uses: adaptive retry
// mode and MaxAttempts unless AWS_RETRY_MODE / AWS_MAX_ATTEMPTS (or shared
// config) set them, and every attempt paced to RequestsPerSecond.
func NewClient(cfg aws.Config, region string, optFns ...func(*identitystore.Options)) *identitystore.Client {
	if cfg.RetryMode == "" {
		cfg.RetryMode = aws.RetryModeAdaptive
	}
	if cfg.RetryMaxAttempts == 0 {
		cfg.RetryMaxAttempts = MaxAttempts
	}
	opts := []func(*identitystore.Options){
		func(o *identitystore.Options) { o.Region = region },
		WithRateLimit(rate.NewLimiter(rate.Limit(RequestsPerSecond), 1)),
	}
	return identitystore.NewFromConfig(cfg, append(opts, optFns...)...)
}

// WithRateLimit waits on l before every attempt of every operation (it runs
// after the SDK's "Retry" middleware), and logs each throttled attempt.
func WithRateLimit(l *rate.Limiter) func(*identitystore.Options) {
	throttle := retry.IsErrorThrottles(retry.DefaultThrottles)
	return func(o *identitystore.Options) {
		o.APIOptions = append(o.APIOptions, func(s *middleware.Stack) error {
			return s.Finalize.Insert(middleware.FinalizeMiddlewareFunc("ssosyncIdentityStoreRateLimit",
				func(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (
					middleware.FinalizeOutput, middleware.Metadata, error) {
					if err := l.Wait(ctx); err != nil {
						return middleware.FinalizeOutput{}, middleware.Metadata{}, err
					}
					out, md, err := next.HandleFinalize(ctx, in)
					if err != nil && throttle.IsErrorThrottle(err) == aws.TrueTernary {
						log.WithFields(log.Fields{"operation": awsmiddleware.GetOperationName(ctx), "error": err}).
							Warn("Identity Store request throttled")
					}
					return out, md, err
				}), "Retry", middleware.After)
		})
	}
}

// CreateGroup creates a group in the identity store
func CreateGroup(ctx context.Context, client interfaces.IdentityStoreAPI, identityStoreID *string, displayName *string) (*identitystore.CreateGroupOutput, error) {
	return client.CreateGroup(ctx, &identitystore.CreateGroupInput{
		IdentityStoreId: identityStoreID,
		DisplayName:     displayName,
	})
}

// CreateGroupMembership creates a group membership
func CreateGroupMembership(ctx context.Context, client interfaces.IdentityStoreAPI, identityStoreID *string, groupID *string, userID *string) (*identitystore.CreateGroupMembershipOutput, error) {
	return client.CreateGroupMembership(ctx, &identitystore.CreateGroupMembershipInput{
		IdentityStoreId: identityStoreID,
		GroupId:         groupID,
		MemberId: &types.MemberIdMemberUserId{
			Value: *userID,
		},
	})
}

// DeleteGroup deletes a group from the identity store
func DeleteGroup(ctx context.Context, client interfaces.IdentityStoreAPI, identityStoreID *string, groupID *string) (*identitystore.DeleteGroupOutput, error) {
	return client.DeleteGroup(ctx, &identitystore.DeleteGroupInput{
		IdentityStoreId: identityStoreID,
		GroupId:         groupID,
	})
}

// DeleteUser deletes a user from the identity store
func DeleteUser(ctx context.Context, client interfaces.IdentityStoreAPI, identityStoreID *string, userID *string) (*identitystore.DeleteUserOutput, error) {
	return client.DeleteUser(ctx, &identitystore.DeleteUserInput{
		IdentityStoreId: identityStoreID,
		UserId:          userID,
	})
}

// IsMemberInGroups checks if a user is a member of specified groups
func IsMemberInGroups(ctx context.Context, client interfaces.IdentityStoreAPI, identityStoreID *string, groupIDs []string, userID *string) (*bool, error) {
	result, err := client.IsMemberInGroups(ctx, &identitystore.IsMemberInGroupsInput{
		IdentityStoreId: identityStoreID,
		MemberId: &types.MemberIdMemberUserId{
			Value: *userID,
		},
		GroupIds: groupIDs,
	})
	if err != nil {
		return nil, err
	}

	// Check if user is member of any of the groups
	for _, membershipResult := range result.Results {
		if membershipResult.MembershipExists {
			return aws.Bool(true), nil
		}
	}
	return aws.Bool(false), nil
}

// ListGroups lists all groups with pagination support
func ListGroups(ctx context.Context, client interfaces.IdentityStoreAPI, identityStoreID *string, converter func(types.Group) *interfaces.Group) ([]*interfaces.Group, error) {
	var groups []*interfaces.Group

	paginator := identitystore.NewListGroupsPaginator(client, &identitystore.ListGroupsInput{
		IdentityStoreId: identityStoreID,
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, group := range page.Groups {
			if convertedGroup := converter(group); convertedGroup != nil {
				groups = append(groups, convertedGroup)
			}
		}
	}

	return groups, nil
}

// ListGroupsPager is a helper function for paginated group listing
func ListGroupsPager(ctx context.Context, paginator *identitystore.ListGroupsPaginator, converter func(types.Group) *interfaces.Group) ([]*interfaces.Group, error) {
	var groups []*interfaces.Group

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, group := range page.Groups {
			if convertedGroup := converter(group); convertedGroup != nil {
				groups = append(groups, convertedGroup)
			}
		}
	}

	return groups, nil
}

// ListUsersPager is a helper function for paginated user listing
func ListUsersPager(ctx context.Context, paginator *identitystore.ListUsersPaginator, converter func(types.User) *interfaces.User) ([]*interfaces.User, error) {
	var users []*interfaces.User

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, user := range page.Users {
			if convertedUser := converter(user); convertedUser != nil {
				users = append(users, convertedUser)
			}
		}
	}

	return users, nil
}

// ListGroupMembershipsPager is a helper function for paginated group membership listing
func ListGroupMembershipsPager(ctx context.Context, paginator *identitystore.ListGroupMembershipsPaginator, converter func(types.GroupMembership) *interfaces.User) ([]*interfaces.User, error) {
	var users []*interfaces.User

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, membership := range page.GroupMemberships {
			if convertedUser := converter(membership); convertedUser != nil {
				users = append(users, convertedUser)
			}
		}
	}

	return users, nil
}

// GetGroupMembershipId gets the membership ID for a user in a group
func GetGroupMembershipId(ctx context.Context, client interfaces.IdentityStoreAPI, identityStoreID *string, groupID *string, userID *string) (*identitystore.GetGroupMembershipIdOutput, error) {
	return client.GetGroupMembershipId(ctx, &identitystore.GetGroupMembershipIdInput{
		IdentityStoreId: identityStoreID,
		GroupId:         groupID,
		MemberId: &types.MemberIdMemberUserId{
			Value: *userID,
		},
	})
}

// DeleteGroupMembership deletes a group membership
func DeleteGroupMembership(ctx context.Context, client interfaces.IdentityStoreAPI, identityStoreID *string, membershipID *string) (*identitystore.DeleteGroupMembershipOutput, error) {
	return client.DeleteGroupMembership(ctx, &identitystore.DeleteGroupMembershipInput{
		IdentityStoreId: identityStoreID,
		MembershipId:    membershipID,
	})
}
