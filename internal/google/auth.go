// Copyright (c) 2020, Amazon.com, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package google

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	admin "google.golang.org/api/admin/directory/v1"
)

// AuthClient is for authenticating with Google and optionally
// getting a token from the web interface interactively
type AuthClient struct {
	logger          *zap.Logger
	credentialsPath string
	tokenPath       string

	config *oauth2.Config
}

// NewAuthClient creates a new AuthClient with the paths given
func NewAuthClient(logger *zap.Logger, credPath string, tokenPath string) (*AuthClient, error) {
	b, err := ioutil.ReadFile(credPath)
	if err != nil {
		logger.Error("Unable to read client secret file", zap.Error(err))
		return nil, err
	}

	config, err := google.ConfigFromJSON(b,
		admin.AdminDirectoryGroupReadonlyScope,
		admin.AdminDirectoryGroupMemberReadonlyScope,
		admin.AdminDirectoryUserReadonlyScope,
	)

	if err != nil {
		logger.Error("unable to parse config from JSON", zap.Error(err))
		return nil, err
	}

	return &AuthClient{
		logger:          logger,
		credentialsPath: credPath,
		tokenPath:       tokenPath,
		config:          config,
	}, nil
}

// GetClient will return the http.Client for the authenticated connection
func (a *AuthClient) GetClient() (*http.Client, error) {
	tok, err := tokenFromFile(a.tokenPath)
	if err != nil {
		return nil, err
	}
	return a.config.Client(context.Background(), tok), nil
}

// GetTokenFromWeb will interactively get a token from Google
func (a *AuthClient) GetTokenFromWeb() *oauth2.Token {
	authURL := a.config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\nAuth Code: ", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		a.logger.Fatal("Unable to read authorization code", zap.Error(err))
	}

	tok, err := a.config.Exchange(context.TODO(), authCode)
	if err != nil {
		a.logger.Fatal("Unable to retrieve token from web", zap.Error(err))
	}

	a.saveToken(tok)

	return tok
}

// saveToken will save the token to the token.json file
func (a *AuthClient) saveToken(token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", a.tokenPath)
	f, err := os.OpenFile(a.tokenPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		a.logger.Fatal("Unable to cache oauth token", zap.Error(err))
	}
	defer f.Close()
	_ = json.NewEncoder(f).Encode(token)
}

// tokenFromFile retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}
