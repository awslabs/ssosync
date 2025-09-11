package aws

import (
	internal_http "ssosync/internal/http"
	"ssosync/internal/interfaces"

	log "github.com/sirupsen/logrus"
)

type dryClient struct {
	c Client
	// users scheduled for creation, but not actually existing in AWS
	virtualUsers map[string]interfaces.User
}

func NewDryClient(c internal_http.Client, config *Config) (Client, error) {
	// create the client by calling NewClient
	client, err := NewClient(c, config)
	if err != nil {
		return nil, err
	}

	return &dryClient{
		c:            client,
		virtualUsers: make(map[string]interfaces.User),
	}, nil
}

func (dc *dryClient) CreateUser(u *interfaces.User) (*interfaces.User, error) {
	log.WithField("user", u.Username).Info("DRY RUN: Would create user")
	dc.virtualUsers[u.Username] = *u
	return u, nil
}

func (dc *dryClient) FindGroupByDisplayName(name string) (*interfaces.Group, error) {
	// this is only used to determine group correlations
	// and for group deletion, so can be straight pass-through
	return dc.c.FindGroupByDisplayName(name)
}

func (dc *dryClient) FindUserByEmail(email string) (*interfaces.User, error) {
	u, err := dc.c.FindUserByEmail(email)
	if err != nil {
		if err != ErrUserNotFound {
			return u, err
		}

		for _, vu := range dc.virtualUsers {
			for _, e := range vu.Emails {
				if e.Value == email {
					log.Debug("User fetch fail, but user found in the virtual state")
					return &vu, nil
				}
			}
		}
		// no match
		return u, err

	}
	return u, nil
}

func (dc *dryClient) UpdateUser(u *interfaces.User) (*interfaces.User, error) {
	log.WithField("user", u.Username).Info("DRY RUN: Would update user")
	dc.virtualUsers[u.Username] = *u
	return u, nil
}

func (dc *dryClient) AddUserToGroup(u *interfaces.User, g *interfaces.Group) error {
	log.WithFields(log.Fields{"user": u.Username, "group": g.DisplayName}).Info("DRY RUN: Would add user to group")
	return nil
}

func (dc *dryClient) RemoveUserFromGroup(u *interfaces.User, g *interfaces.Group) error {
	log.WithFields(log.Fields{"user": u.Username, "group": g.DisplayName}).Info("DRY RUN: Would remove user from group")
	return nil
}

func (dc *dryClient) CreateGroup(g *interfaces.Group) (*interfaces.Group, error) {
	log.WithField("group", g.DisplayName).Info("DRY RUN: Would create group")
	return g, nil
}

func (dc *dryClient) UpdateGroup(g *interfaces.Group) (*interfaces.Group, error) {
	log.WithField("group", g.DisplayName).Info("DRY RUN: Would update group")
	return g, nil
}

func (dc *dryClient) DeleteGroup(g *interfaces.Group) error {
	log.WithField("group", g.DisplayName).Info("DRY RUN: Would delete group")
	return nil
}

func (dc *dryClient) DeleteUser(u *interfaces.User) error {
	log.WithField("user", u.Username).Info("DRY RUN: Would delete user")
	delete(dc.virtualUsers, u.Username)
	return nil
}
