package internal

import (
	"go.uber.org/zap"
	admin "google.golang.org/api/admin/directory/v1"

	"github.com/awslabs/ssosync/internal/aws"
	"github.com/awslabs/ssosync/internal/google"
)

// ISyncGSuite is the interface for synchronising users/groups
type ISyncGSuite interface {
	SyncUsers() error
	SyncGroups() error
}

// SyncGSuite is an object type that will synchronise real users and groups
type SyncGSuite struct {
	aws    aws.IClient
	google google.IClient
	logger *zap.Logger

	users map[string]*aws.User
}

// New will create a new SyncGSuite object
func New(logger *zap.Logger, a aws.IClient, g google.IClient) ISyncGSuite {
	return &SyncGSuite{
		aws:    a,
		google: g,
		logger: logger,
		users:  make(map[string]*aws.User),
	}
}

// SyncUsers will Sync Google Users to AWS SSO SCIM
func (s *SyncGSuite) SyncUsers() error {
	s.logger.Info("Start user sync")
	s.logger.Debug("Get AWS Users")
	awsUsers, err := s.aws.GetUsers()
	if err != nil {
		return err
	}
	s.logger.Debug("Get Google Users")
	googleUsers, err := s.google.GetUsers()
	if err != nil {
		return err
	}

	for _, u := range googleUsers {
		logUser := []zap.Field{
			zap.String("email", u.PrimaryEmail),
		}

		s.logger.Debug("Check user", logUser...)
		if awsUser, ok := (*awsUsers)[u.PrimaryEmail]; ok {
			s.logger.Debug("Found user", logUser...)
			s.users[awsUser.Username] = &awsUser
		} else {
			s.logger.Info("Create user in AWS", logUser...)
			newUser, err := s.aws.CreateUser(aws.NewUser(
				u.Name.GivenName,
				u.Name.FamilyName,
				u.PrimaryEmail,
			))
			if err != nil {
				return err
			}
			s.users[newUser.Username] = newUser
		}
	}

	s.logger.Info("Clean up AWS Users")
	for _, u := range *awsUsers {
		if _, ok := s.users[u.Username]; !ok {
			s.logger.Info("Delete User in AWS", zap.String("email", u.Username))
			err := s.aws.DeleteUser(&u)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// SyncGroups will sync groups from Google -> AWS SSO
func (s *SyncGSuite) SyncGroups() error {
	s.logger.Info("Start group sync")

	s.logger.Debug("Get AWS Groups")
	awsGroups, err := s.aws.GetGroups()
	if err != nil {
		return err
	}

	s.logger.Debug("Get Google Groups")
	googleGroups, err := s.google.GetGroups()
	if err != nil {
		return err
	}

	correlatedGroups := make(map[string]*aws.Group)

	for _, g := range googleGroups {
		logGroup := []zap.Field{
			zap.String("group", g.Name),
		}

		s.logger.Debug("Check group", logGroup...)

		var group *aws.Group

		if awsGroup, ok := (*awsGroups)[g.Name]; ok {
			s.logger.Debug("Found group", logGroup...)
			correlatedGroups[awsGroup.DisplayName] = &awsGroup
			group = &awsGroup
		} else {
			s.logger.Info("Creating group in AWS", logGroup...)
			newGroup, err := s.aws.CreateGroup(aws.NewGroup(g.Name))
			if err != nil {
				return err
			}
			correlatedGroups[newGroup.DisplayName] = newGroup
			group = newGroup
		}

		groupMembers, err := s.google.GetGroupMembers(g)
		if err != nil {
			return err
		}

		memberList := make(map[string]*admin.Member)

		s.logger.Info("Start group user sync", logGroup...)

		for _, m := range groupMembers {
			if _, ok := s.users[m.Email]; ok {
				memberList[m.Email] = m
			}
		}

		for _, u := range s.users {
			logDetail := append(logGroup, zap.String("user", u.Username))

			s.logger.Debug("Checking user is in group already", logDetail...)
			b, err := s.aws.IsUserInGroup(u, group)
			if err != nil {
				return err
			}

			if _, ok := memberList[u.Username]; ok {
				if !b {
					s.logger.Info("Adding user to group", logDetail...)
					err := s.aws.AddUserToGroup(u, group)
					if err != nil {
						return err
					}
				}
			} else {
				if b {
					s.logger.Info("Removing user from group", logDetail...)
					err := s.aws.RemoveUserFromGroup(u, group)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	s.logger.Info("Clean up AWS groups")
	for _, g := range *awsGroups {
		if _, ok := correlatedGroups[g.DisplayName]; !ok {
			s.logger.Info("Delete Group in AWS", zap.String("group", g.DisplayName))
			err := s.aws.DeleteGroup(&g)
			if err != nil {
				return err
			}
		}
	}

	s.logger.Info("Done sync groups")
	return nil
}
