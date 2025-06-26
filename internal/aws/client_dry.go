package aws

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"

	log "github.com/sirupsen/logrus"
)


type dryClient struct {
    // TODO: can I specify that I want lovercase "c" client? The concrete implementation
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
    // TODO: add user to virtualUsers by id
    return u, nil;
}

func (dc *dryClient) FindGroupByDisplayName(name string) (*Group, error) {
    // this is only used to determie group corelations
    // and for group deletion, so can be straight pass-through
    return dc.c.FindGroupByDisplayName(name)
}

func (dc *dryClient) FindUserByEmail(email string) (*User, error) {
    // TODO: handle error from dryState
    return dc.c.FindUserByEmail(email)
}

func (dc *dryClient) UpdateUser(u *User) (*User, error) {
    // TODO: update user in virtualUselrs by id
    return u, nil;
}
