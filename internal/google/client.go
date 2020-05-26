package google

import (
	"context"
	"net/http"

	"go.uber.org/zap"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
)

// IClient is the Interface for the Client
type IClient interface {
	GetUsers() ([]*admin.User, error)
	GetGroups() ([]*admin.Group, error)
	GetGroupMembers(*admin.Group) ([]*admin.Member, error)
}

// Client is the Google Apps for Domains Client
type Client struct {
	logger  *zap.Logger
	client  *http.Client
	service *admin.Service
}

// NewClient creates a new client for Google's Admin API
func NewClient(logger *zap.Logger, client *AuthClient) (IClient, error) {
	c, err := client.GetClient()
	if err != nil {
		return nil, err
	}

	srv, err := admin.NewService(context.TODO(), option.WithHTTPClient(c))
	if err != nil {
		return nil, err
	}

	return &Client{
		logger:  logger,
		client:  c,
		service: srv,
	}, nil
}

// GetUsers will get the users from Google's Admin API
func (c *Client) GetUsers() (u []*admin.User, err error) {
	u = make([]*admin.User, 0)
	err = c.service.Users.List().Customer("my_customer").Pages(context.TODO(), func(users *admin.Users) error {
		u = append(u, users.Users...)
		return nil
	})

	return
}

// GetGroups will get the groups from Google's Admin API
func (c *Client) GetGroups() (g []*admin.Group, err error) {
	g = make([]*admin.Group, 0)
	err = c.service.Groups.List().Customer("my_customer").Pages(context.TODO(), func(groups *admin.Groups) error {
		g = append(g, groups.Groups...)
		return nil
	})

	return
}

// GetGroupMembers will get the members of the group specified
func (c *Client) GetGroupMembers(g *admin.Group) (m []*admin.Member, err error) {
	m = make([]*admin.Member, 0)
	err = c.service.Members.List(g.Id).Pages(context.TODO(), func(members *admin.Members) error {
		m = append(m, members.Members...)
		return nil
	})

	return
}
