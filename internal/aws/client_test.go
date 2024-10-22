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
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/awslabs/ssosync/internal/aws/mock"
)

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

type httpReqMatcher struct {
	httpReq *http.Request
	headers map[string]string
	body    string
}

func (r *httpReqMatcher) Matches(req interface{}) bool {
	m, ok := req.(*http.Request)
	if !ok {
		return false
	}

	for k, v := range r.headers {
		if m.Header.Get(k) != v {
			return false
		}
	}

	if m.Body != nil {
		got, _ := ioutil.ReadAll(m.Body)
		if string(got) != r.body {
			return false
		}
	}

	return m.URL.String() == r.httpReq.URL.String() && m.Method == r.httpReq.Method
}

func (r *httpReqMatcher) String() string {
	return fmt.Sprintf("is %s", r.httpReq.URL)
}

func TestNewClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	x := mock.NewIHTTPClient(ctrl)

	c, err := NewClient(x, &Config{
		Endpoint: ":foo",
		Token:    "bearerToken",
	})
	assert.Error(t, err)
	assert.Nil(t, c)
}

func TestSendRequestBadUrl(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	x := mock.NewIHTTPClient(ctrl)

	c, err := NewClient(x, &Config{
		Endpoint: "https://scim.example.com/",
		Token:    "bearerToken",
	})
	assert.NoError(t, err)
	cc := c.(*client)

	r, err := cc.sendRequest(http.MethodGet, ":foo")
	assert.Error(t, err)
	assert.Nil(t, r)
}

func TestSendRequestBadStatusCode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	x := mock.NewIHTTPClient(ctrl)

	c, err := NewClient(x, &Config{
		Endpoint: "https://scim.example.com/",
		Token:    "bearerToken",
	})
	assert.NoError(t, err)
	cc := c.(*client)

	calledURL, _ := url.Parse("https://scim.example.com/")

	req := httpReqMatcher{httpReq: &http.Request{
		URL:    calledURL,
		Method: http.MethodGet,
	}}

	x.EXPECT().Do(&req).MaxTimes(1).Return(&http.Response{
		Status:     "ERROR",
		StatusCode: 500,
		Body:       nopCloser{bytes.NewBufferString("")},
	}, nil)

	_, err = cc.sendRequest(http.MethodGet, "https://scim.example.com/")
	assert.Error(t, err)
}

func TestSendRequestCheckAuthHeader(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	x := mock.NewIHTTPClient(ctrl)

	c, err := NewClient(x, &Config{
		Endpoint: "https://scim.example.com/",
		Token:    "bearerToken",
	})
	assert.NoError(t, err)
	cc := c.(*client)

	calledURL, _ := url.Parse("https://scim.example.com/")

	req := httpReqMatcher{
		httpReq: &http.Request{
			URL:    calledURL,
			Method: http.MethodGet,
		},
		headers: map[string]string{
			"Authorization": "Bearer bearerToken",
		},
	}

	x.EXPECT().Do(&req).MaxTimes(1).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBufferString("")},
	}, nil)

	_, err = cc.sendRequest(http.MethodGet, "https://scim.example.com/")
	assert.NoError(t, err)
}

func TestSendRequestWithBodyCheckHeaders(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	x := mock.NewIHTTPClient(ctrl)

	c, err := NewClient(x, &Config{
		Endpoint: "https://scim.example.com/",
		Token:    "bearerToken",
	})
	assert.NoError(t, err)
	cc := c.(*client)

	calledURL, _ := url.Parse("https://scim.example.com/")

	req := httpReqMatcher{
		httpReq: &http.Request{
			URL:    calledURL,
			Method: http.MethodPost,
		},
		headers: map[string]string{
			"Authorization": "Bearer bearerToken",
			"Content-Type":  "application/scim+json",
		},
		body: "{\"schemas\":null,\"userName\":\"\",\"name\":{\"familyName\":\"\",\"givenName\":\"\"},\"displayName\":\"\",\"active\":false,\"emails\":null,\"addresses\":null}",
	}

	x.EXPECT().Do(&req).MaxTimes(1).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBufferString("")},
	}, nil)

	_, err = cc.sendRequestWithBody(http.MethodPost, "https://scim.example.com/", &User{})
	assert.NoError(t, err)
}

func TestClient_FindUserByEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	x := mock.NewIHTTPClient(ctrl)

	c, err := NewClient(x, &Config{
		Endpoint: "https://scim.example.com/",
		Token:    "bearerToken",
	})
	assert.NoError(t, err)

	calledURL, _ := url.Parse("https://scim.example.com/Users")

	filter := "userName eq \"test@example.com\""

	q := calledURL.Query()
	q.Add("filter", filter)

	calledURL.RawQuery = q.Encode()

	req := httpReqMatcher{
		httpReq: &http.Request{
			URL:    calledURL,
			Method: http.MethodGet,
		},
	}

	// Error in response
	x.EXPECT().Do(&req).MaxTimes(1).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBufferString("")},
	}, nil)

	u, err := c.FindUserByEmail("test@example.com")
	assert.Nil(t, u)
	assert.Error(t, err)

	// False
	r := &UserFilterResults{
		TotalResults: 0,
	}
	falseResult, _ := json.Marshal(r)

	x.EXPECT().Do(&req).MaxTimes(1).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBuffer(falseResult)},
	}, nil)

	u, err = c.FindUserByEmail("test@example.com")
	assert.Nil(t, u)
	assert.Error(t, err)

	// True
	r = &UserFilterResults{
		TotalResults: 1,
		Resources: []User{
			{
				Username: "test@example.com",
			},
		},
	}
	trueResult, _ := json.Marshal(r)
	x.EXPECT().Do(&req).MaxTimes(1).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBuffer(trueResult)},
	}, nil)

	u, err = c.FindUserByEmail("test@example.com")
	assert.NotNil(t, u)
	assert.NoError(t, err)
}

func TestClient_FindGroupByDisplayName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	x := mock.NewIHTTPClient(ctrl)

	c, err := NewClient(x, &Config{
		Endpoint: "https://scim.example.com/",
		Token:    "bearerToken",
	})
	assert.NoError(t, err)

	calledURL, _ := url.Parse("https://scim.example.com/Groups")

	filter := "displayName eq \"testGroup\""

	q := calledURL.Query()
	q.Add("filter", filter)

	calledURL.RawQuery = q.Encode()

	req := httpReqMatcher{
		httpReq: &http.Request{
			URL:    calledURL,
			Method: http.MethodGet,
		},
	}

	// Error in response
	x.EXPECT().Do(&req).MaxTimes(1).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBufferString("")},
	}, nil)

	u, err := c.FindGroupByDisplayName("testGroup")
	assert.Nil(t, u)
	assert.Error(t, err)

	// False
	r := &GroupFilterResults{
		TotalResults: 0,
	}
	falseResult, _ := json.Marshal(r)

	x.EXPECT().Do(&req).MaxTimes(1).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBuffer(falseResult)},
	}, nil)

	u, err = c.FindGroupByDisplayName("testGroup")
	assert.Nil(t, u)
	assert.Error(t, err)

	// True
	r = &GroupFilterResults{
		TotalResults: 1,
		Resources: []Group{
			{
				DisplayName: "testGroup",
			},
		},
	}
	trueResult, _ := json.Marshal(r)
	x.EXPECT().Do(&req).MaxTimes(1).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBuffer(trueResult)},
	}, nil)

	u, err = c.FindGroupByDisplayName("testGroup")
	assert.NotNil(t, u)
	assert.NoError(t, err)
}

func TestClient_CreateUser(t *testing.T) {
	nu := NewUser("Lee", "Packham", "test@example.com", true)
	nuResult := *nu
	nuResult.ID = "userId"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	x := mock.NewIHTTPClient(ctrl)

	c, err := NewClient(x, &Config{
		Endpoint: "https://scim.example.com/",
		Token:    "bearerToken",
	})
	assert.NoError(t, err)

	calledURL, _ := url.Parse("https://scim.example.com/Users")

	requestJSON, _ := json.Marshal(nu)

	req := httpReqMatcher{
		httpReq: &http.Request{
			URL:    calledURL,
			Method: http.MethodPost,
		},
		body: string(requestJSON),
	}

	response, _ := json.Marshal(nuResult)

	x.EXPECT().Do(&req).MaxTimes(1).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBuffer(response)},
	}, nil)

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

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	x := mock.NewIHTTPClient(ctrl)

	c, err := NewClient(x, &Config{
		Endpoint: "https://scim.example.com/",
		Token:    "bearerToken",
	})
	assert.NoError(t, err)

	calledURL, _ := url.Parse("https://scim.example.com/Users/userId")

	requestJSON, _ := json.Marshal(nu)

	req := httpReqMatcher{
		httpReq: &http.Request{
			URL:    calledURL,
			Method: http.MethodPut,
		},
		body: string(requestJSON),
	}

	response, _ := json.Marshal(nuResult)

	x.EXPECT().Do(&req).MaxTimes(1).Return(&http.Response{
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
