package aws

import (
	log "github.com/sirupsen/logrus"
)


type dryClient struct {
    c Client
    // users scheduled for creation, but not actually existing in AWS
    virtualUsers map[string]User
}

func NewDryClient(c HTTPClient, config *Config) (Client, error) {
    // create the client by calling NewClient
    client, err := NewClient(c, config)
    if err != nil {
        return nil, err
    }
    
    return &dryClient{
        c: client,
        virtualUsers: make(map[string]User),
    }, nil
}

func (dc *dryClient) CreateUser(u *User) (*User, error) {
    dc.virtualUsers[u.Username] = *u
    return u, nil
}

func (dc *dryClient) FindGroupByDisplayName(name string) (*Group, error) {
    // this is only used to determine group correlations
    // and for group deletion, so can be straight pass-through
    return dc.c.FindGroupByDisplayName(name)
}

func (dc *dryClient) FindUserByEmail(email string) (*User, error) {
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

func (dc *dryClient) UpdateUser(u *User) (*User, error) {
    dc.virtualUsers[u.Username] = *u
    return u, nil
}
