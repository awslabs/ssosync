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

package aws

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"testing"

	"github.com/awslabs/ssosync/internal/interfaces"
	"github.com/awslabs/ssosync/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

var scheme = "https"
var host = "scim.example.com"

var baseUrl = scheme + "://" + host

func requestMatcher(expectedMethod string, expectedPath string, expectedRawQuery string, expectedBody []byte) func(req *http.Request) bool {
	return func(req *http.Request) bool {
		if req.Method != expectedMethod {
			log.Fatalf("Incorrect method %s <> %s", req.Method, expectedMethod)
			return false
		}
		if req.URL.Scheme != scheme {
			log.Fatalf("Incorrect scheme  %s <> %s", req.URL.Scheme, scheme)
			return false
		}
		if req.URL.Host != host {
			log.Fatalf("Incorrect host  %s <> %s", req.URL.Host, host)
			return false
		}
		if req.URL.Path != expectedPath {
			log.Fatalf("Incorrect path  %s <> %s", req.URL.Path, expectedPath)
			return false
		}
		if req.URL.RawQuery != expectedRawQuery {
			log.Fatalf("Incorrect rawQuery  %s <> %s", req.URL.RawQuery, expectedRawQuery)
			return false
		}
		// for key, values := range req.Header {
		// 	for _, value := range values {
		// 		assert.Equal(t, value, actual.Header.Get(key))
		// 	}
		// }
		if expectedBody != nil {
			reqBody, err := io.ReadAll(req.Body)
			if err != nil {
				fmt.Println(err)
				return false
			}
			if reqBody != nil && !bytes.Equal(expectedBody, reqBody) {
				log.Println("Incorrect body")
				log.Println(string(expectedBody))
				log.Println("============================")
				log.Println(string(reqBody))
				return false
			}
		}
		return true
	}
}

func TestSendRequestBadUrl(t *testing.T) {
	x := mocks.NewMockClient(t)
	x.EXPECT().Do(mock.Anything).Return(nil, errors.New("BadURL")).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})

	assert.NoError(t, err)
	cc := c.(*client)

	r, err := cc.get(":foo", nil)
	assert.Error(t, err)
	assert.Nil(t, r)
}

func TestSendRequestBadStatusCode(t *testing.T) {
	x := mocks.NewMockClient(t)
	x.EXPECT().Do(mock.Anything).Return(&http.Response{
		Status:     "ERROR",
		StatusCode: 500,
	}, nil).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})

	assert.NoError(t, err)
	cc := c.(*client)

	r, err := cc.get(":foo", nil)
	assert.Error(t, err)
	assert.Nil(t, r)

}

func TestPrepareRequest(t *testing.T) {
	x := mocks.NewMockClient(t)

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})

	assert.NoError(t, err)
	cc := c.(*client)

	r, err := cc.prepareRequest(http.MethodGet, "/abcd", nil)
	assert.NoError(t, err)
	assert.Equal(t, "Bearer bearerToken", r.Header.Get("Authorization"))
	assert.Equal(t, "application/scim+json", r.Header.Get("Content-Type"))
	assert.Equal(t, http.MethodGet, r.Method)
	assert.Equal(t, "https://scim.example.com/abcd", r.URL.String())

}

func TestPost(t *testing.T) {
	x := mocks.NewMockClient(t)
	body := &interfaces.User{Schemas: nil, Username: "", DisplayName: "", Active: false, Emails: nil, Addresses: nil}
	by, _ := json.Marshal(body)
	json.Unmarshal(by, body)
	by, _ = json.Marshal(body)

	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodPost, "/foo", "", by))).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBufferString("")},
	}, nil).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})

	assert.NoError(t, err)
	cc := c.(*client)

	_, err = cc.post("/foo", body)
	assert.NoError(t, err)
}

func TestPut(t *testing.T) {
	x := mocks.NewMockClient(t)
	body := &interfaces.User{Schemas: nil, Username: "", DisplayName: "", Active: false, Emails: nil, Addresses: nil}
	by, _ := json.Marshal(body)
	json.Unmarshal(by, body)
	by, _ = json.Marshal(body)

	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodPut, "/foo", "", by))).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBufferString("")},
	}, nil).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})

	assert.NoError(t, err)
	cc := c.(*client)

	_, err = cc.put("/foo", body)
	assert.NoError(t, err)
}

func TestGet(t *testing.T) {
	x := mocks.NewMockClient(t)

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})

	assert.NoError(t, err)
	cc := c.(*client)

	r, err := cc.prepareRequest(http.MethodGet, "/abcd", nil)
	assert.NoError(t, err)
	assert.Equal(t, "Bearer bearerToken", r.Header.Get("Authorization"))
	assert.Equal(t, http.MethodGet, r.Method)
	assert.Equal(t, "https://scim.example.com/abcd", r.URL.String())

	filter := "userName eq \"test@example.com\""
	r.URL.RawQuery = "filter=" + url.QueryEscape(filter)
	x.EXPECT().Do(r).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBufferString("")},
	}, nil).Once()

	_, err = cc.get("/abcd", func(r *http.Request) {
		r.URL.RawQuery = "filter=" + url.QueryEscape(filter)
	})
	assert.NoError(t, err)
}

func TestClient_FindUserByEmail(t *testing.T) {
	// False
	falseResult, _ := json.Marshal(&interfaces.UserFilterResults{
		TotalResults: 0,
	})

	rq := "filter=userName+eq+%22test%40example.com%22"

	x := mocks.NewMockClient(t)
	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodGet, "/Users", rq, nil))).Once().Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBuffer(falseResult)},
	}, nil).Once()

	trueResult, _ := json.Marshal(&interfaces.UserFilterResults{
		TotalResults: 1,
		Resources: []interfaces.User{
			{
				Username: "test@example.com",
			},
		},
	})
	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodGet, "/Users", rq, nil))).Once().Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBuffer(trueResult)},
	}, nil)

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})

	assert.NoError(t, err)
	cc := c.(*client)

	u, err := cc.FindUserByEmail("test@example.com")
	assert.EqualError(t, err, ErrUserNotFound.Error())
	assert.Nil(t, u)

	// True

	u, err = c.FindUserByEmail("test@example.com")
	assert.NotNil(t, u)
	assert.NoError(t, err)
}

func TestClient_FindGroupByDisplayName(t *testing.T) {

	falseResult, _ := json.Marshal(&interfaces.GroupFilterResults{
		TotalResults: 0,
	})
	rq := "filter=displayName+eq+%22testGroup%22"
	x := mocks.NewMockClient(t)
	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodGet, "/Groups", rq, nil))).Once().Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBuffer(falseResult)},
	}, nil).Once()

	trueResult, _ := json.Marshal(&interfaces.GroupFilterResults{
		TotalResults: 1,
		Resources: []interfaces.Group{
			{
				DisplayName: "testGroup",
			},
		},
	})
	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodGet, "/Groups", rq, nil))).Once().Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBuffer(trueResult)},
	}, nil)

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})

	assert.NoError(t, err)

	g, err := c.FindGroupByDisplayName("testGroup")
	assert.Nil(t, g)
	assert.EqualError(t, err, ErrGroupNotFound.Error())

	g, err = c.FindGroupByDisplayName("testGroup")
	assert.NotNil(t, g)
	assert.NoError(t, err)
}

func TestClient_CreateUser(t *testing.T) {
	nu := NewUser("Lee", "Packham", "test@example.com", true)
	nuResult := *nu
	nuResult.ID = "userId"
	// trick to ensure after marshalling we have the correct string
	requestJSON, _ := json.Marshal(nu)
	json.Unmarshal(requestJSON, nu)
	requestJSON, _ = json.Marshal(nu)

	response, _ := json.Marshal(nuResult)

	x := mocks.NewMockClient(t)
	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodPost, "/Users", "", requestJSON))).Once().Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBuffer(response)},
	}, nil).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})

	assert.NoError(t, err)

	r, err := c.CreateUser(nu)
	assert.NotNil(t, r)
	assert.NoError(t, err)

	if r != nil {
		assert.Equal(t, *r, nuResult)
	}
}

func TestClient_UpdateUser(t *testing.T) {
	nu := UpdateUser("userId", "Lee", "Packham", "test@example.com", true)
	nuResult := *nu
	nuResult.ID = "userId"
	requestJSON, _ := json.Marshal(nu)
	json.Unmarshal(requestJSON, nu)
	requestJSON, _ = json.Marshal(nu)
	response, _ := json.Marshal(nuResult)

	x := mocks.NewMockClient(t)

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	assert.NoError(t, err)

	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodPut, "/Users/userId", "", requestJSON))).Times(1).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBuffer(response)},
	}, nil)

	r, err := c.UpdateUser(nu)
	assert.NotNil(t, r)
	assert.NoError(t, err)

	if r != nil {
		assert.Equal(t, *r, nuResult)
	}
}
