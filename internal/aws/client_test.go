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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"ssosync/internal/interfaces"
	"ssosync/internal/mocks"

	"github.com/aws/aws-sdk-go-v2/service/identitystore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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
			return false
		}
		if req.URL.Scheme != scheme {
			return false
		}
		if req.URL.Host != host {
			return false
		}
		if req.URL.Path != expectedPath {
			return false
		}
		if req.URL.RawQuery != expectedRawQuery {
			return false
		}

		if expectedBody != nil {
			reqBody, err := io.ReadAll(req.Body)
			if err != nil {
				return false
			}
			// For JSON comparison, we need to compare the actual JSON structure
			if len(expectedBody) > 0 && len(reqBody) > 0 {
				var expectedJSON, actualJSON interface{}
				if json.Unmarshal(expectedBody, &expectedJSON) == nil &&
					json.Unmarshal(reqBody, &actualJSON) == nil {
					return fmt.Sprintf("%v", expectedJSON) == fmt.Sprintf("%v", actualJSON)
				}
			}
			return bytes.Equal(expectedBody, reqBody)
		}
		return true
	}
}

func TestSendRequestBadUrl(t *testing.T) {
	x := mocks.NewMockHttpClient(t)
	x.EXPECT().Do(mock.Anything).Return(nil, errors.New("BadURL")).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})

	require.NoError(t, err)
	cc := c.(*client)

	r, err := cc.get("/:foo", nil)
	require.Error(t, err)
	assert.Nil(t, r)
}

func TestSendRequestBadStatusCode(t *testing.T) {
	x := mocks.NewMockHttpClient(t)
	x.EXPECT().Do(mock.Anything).Return(&http.Response{
		Status:     "ERROR",
		StatusCode: 500,
	}, nil).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})

	require.NoError(t, err)
	cc := c.(*client)

	r, err := cc.get("/:foo", nil)
	require.Error(t, err)
	assert.Nil(t, r)

}

func TestPrepareRequest(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})

	require.NoError(t, err)
	cc := c.(*client)

	r, err := cc.prepareRequest(http.MethodGet, "/abcd", nil)
	require.NoError(t, err)
	assert.Equal(t, "Bearer bearerToken", r.Header.Get("Authorization"))
	assert.Equal(t, "application/scim+json", r.Header.Get("Content-Type"))
	assert.Equal(t, http.MethodGet, r.Method)
	assert.Equal(t, "https://scim.example.com/abcd", r.URL.String())

}

func TestPost(t *testing.T) {
	x := mocks.NewMockHttpClient(t)
	body := &interfaces.User{Schemas: nil, Username: "", DisplayName: "", Active: false, Emails: nil, Addresses: nil}
	by, err := json.Marshal(body)
	require.NoError(t, err)
	err = json.Unmarshal(by, body)
	require.NoError(t, err)
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

	require.NoError(t, err)
	cc := c.(*client)

	_, err = cc.post("/foo", body)
	require.NoError(t, err)
}

func TestPut(t *testing.T) {
	x := mocks.NewMockHttpClient(t)
	body := &interfaces.User{Schemas: nil, Username: "", DisplayName: "", Active: false, Emails: nil, Addresses: nil}
	by, err := json.Marshal(body)
	require.NoError(t, err)
	err = json.Unmarshal(by, body)
	require.NoError(t, err)
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

	require.NoError(t, err)
	cc := c.(*client)

	_, err = cc.put("/foo", body)
	require.NoError(t, err)
}

func TestGet(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})

	require.NoError(t, err)
	cc := c.(*client)

	r, err := cc.prepareRequest(http.MethodGet, "/abcd", nil)
	require.NoError(t, err)
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
	require.NoError(t, err)
}

func TestClient_FindUserByEmail(t *testing.T) {
	// False
	falseResult, _ := json.Marshal(&interfaces.UserFilterResults{
		TotalResults: 0,
	})

	rq := "filter=userName+eq+%22test%40example.com%22"

	x := mocks.NewMockHttpClient(t)
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

	require.NoError(t, err)
	cc := c.(*client)

	u, err := cc.FindUserByEmail("test@example.com")
	require.EqualError(t, err, ErrUserNotFound.Error())
	assert.Nil(t, u)

	// True

	u, err = c.FindUserByEmail("test@example.com")
	assert.NotNil(t, u)
	require.NoError(t, err)
}

func TestClient_FindGroupByDisplayName(t *testing.T) {

	falseResult, _ := json.Marshal(&interfaces.GroupFilterResults{
		TotalResults: 0,
	})
	rq := "filter=displayName+eq+%22testGroup%22"
	x := mocks.NewMockHttpClient(t)
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

	require.NoError(t, err)

	g, err := c.FindGroupByDisplayName("testGroup")
	assert.Nil(t, g)
	require.EqualError(t, err, ErrGroupNotFound.Error())

	g, err = c.FindGroupByDisplayName("testGroup")
	assert.NotNil(t, g)
	require.NoError(t, err)
}

func TestClient_DeleteGroup(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	g := &interfaces.Group{
		ID:          "groupId",
		DisplayName: "testGroup",
	}

	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodDelete, "/Groups/groupId", "", nil))).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBufferString("")},
	}, nil).Once()

	err = c.DeleteGroup(g)
	assert.NoError(t, err)

	// Test no group specified
	err = c.DeleteGroup(nil)
	assert.EqualError(t, err, ErrGroupNotSpecified.Error())
}

func TestClient_DeleteUser(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	u := &interfaces.User{
		ID:       "userId",
		Username: "test@example.com",
	}

	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodDelete, "/Users/userId", "", nil))).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBufferString("")},
	}, nil).Once()

	err = c.DeleteUser(u)
	assert.NoError(t, err)

	// Test no user specified
	err = c.DeleteUser(nil)
	assert.EqualError(t, err, ErrUserNotSpecified.Error())
}

func TestClient_CreateUser(t *testing.T) {
	nu := NewUser("Lee", "Packham", "test@example.com", true, "google_id")
	nuResult := *nu
	nuResult.ID = "userId"
	// trick to ensure after marshalling we have the correct string
	requestJSON, err := json.Marshal(nu)
	require.NoError(t, err)

	err = json.Unmarshal(requestJSON, nu)
	require.NoError(t, err)

	requestJSON, err = json.Marshal(nu)
	require.NoError(t, err)

	response, _ := json.Marshal(nuResult)

	x := mocks.NewMockHttpClient(t)
	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodPost, "/Users", "", requestJSON))).Once().Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBuffer(response)},
	}, nil).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})

	require.NoError(t, err)

	r, err := c.CreateUser(nu)
	assert.NotNil(t, r)
	require.NoError(t, err)

	if r != nil {
		assert.Equal(t, *r, nuResult)
	}
}

func TestClient_UpdateUser(t *testing.T) {
	nu := UpdateUser("userId", "Lee", "Packham", "test@example.com", true, "google_id")
	nuResult := *nu
	nuResult.ID = "userId"
	requestJSON, err := json.Marshal(nu)
	require.NoError(t, err)

	err = json.Unmarshal(requestJSON, nu)
	require.NoError(t, err)

	requestJSON, err = json.Marshal(nu)
	require.NoError(t, err)

	response, err := json.Marshal(nuResult)
	require.NoError(t, err)

	x := mocks.NewMockHttpClient(t)

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodPut, "/Users/userId", "", requestJSON))).Times(1).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBuffer(response)},
	}, nil)

	r, err := c.UpdateUser(nu)
	assert.NotNil(t, r)
	require.NoError(t, err)

	if r != nil {
		assert.Equal(t, *r, nuResult)
	}
}

func TestClient_CreateGroup(t *testing.T) {
	ng := NewGroup("test_group@example.com", "google_id")
	ngResult := *ng
	ngResult.ID = "groupId"

	requestJSON, err := json.Marshal(ng)
	require.NoError(t, err)

	response, err := json.Marshal(ngResult)
	require.NoError(t, err)

	x := mocks.NewMockHttpClient(t)
	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodPost, "/Groups", "", requestJSON))).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBuffer(response)},
	}, nil).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	r, err := c.CreateGroup(ng)
	assert.NotNil(t, r)
	require.NoError(t, err)

	if r != nil {
		assert.Equal(t, ngResult, *r)
	}
}

func TestClient_AddUserToGroup(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	g := &interfaces.Group{
		ID:          "groupId",
		DisplayName: "testGroup",
	}

	u := &interfaces.User{
		ID:       "userId",
		Username: "test@example.com",
	}

	expectedBody := `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"add","path":"members","value":[{"value":"userId"}]}]}`

	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodPatch, "/Groups/groupId", "", []byte(expectedBody)))).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBufferString("")},
	}, nil).Once()

	err = c.AddUserToGroup(u, g)
	assert.NoError(t, err)

	// Test error cases
	err = c.AddUserToGroup(nil, g)
	assert.EqualError(t, err, ErrUserNotSpecified.Error())

	err = c.AddUserToGroup(u, nil)
	assert.EqualError(t, err, ErrGroupNotSpecified.Error())
}

func TestClient_RemoveUserFromGroup(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	g := &interfaces.Group{
		ID:          "groupId",
		DisplayName: "testGroup",
	}

	u := &interfaces.User{
		ID:       "userId",
		Username: "test@example.com",
	}

	expectedBody := `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"remove","path":"members","value":[{"value":"userId"}]}]}`

	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodPatch, "/Groups/groupId", "", []byte(expectedBody)))).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBufferString("")},
	}, nil).Once()

	err = c.RemoveUserFromGroup(u, g)
	assert.NoError(t, err)

	// Test error cases
	err = c.RemoveUserFromGroup(nil, g)
	assert.EqualError(t, err, ErrUserNotSpecified.Error())

	err = c.RemoveUserFromGroup(u, nil)
	assert.EqualError(t, err, ErrGroupNotSpecified.Error())
}

// Additional comprehensive test cases

func TestNewClient_InvalidURL(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	// Test invalid URL
	_, err := NewClient(x, &Config{
		Endpoint: "invalid-url",
		Token:    "bearerToken",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid URL")

	// Test non-HTTPS URL
	_, err = NewClient(x, &Config{
		Endpoint: "http://example.com",
		Token:    "bearerToken",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid URL")
}

func TestClient_PrepareRequestWithBody(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)
	cc := c.(*client)

	body := map[string]string{"test": "value"}
	r, err := cc.prepareRequest(http.MethodPost, "/test", body)
	require.NoError(t, err)

	assert.Equal(t, "Bearer bearerToken", r.Header.Get("Authorization"))
	assert.Equal(t, "application/scim+json", r.Header.Get("Content-Type"))
	assert.Equal(t, http.MethodPost, r.Method)
	assert.Equal(t, "https://scim.example.com/test", r.URL.String())
}

func TestClient_PrepareRequestMarshalError(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)
	cc := c.(*client)

	// Use a channel which cannot be marshaled to JSON
	invalidBody := make(chan int)
	_, err = cc.prepareRequest(http.MethodPost, "/test", invalidBody)
	assert.Error(t, err)
}

func TestClient_GetWithNilBody(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodGet, "/test", "", nil))).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nil,
	}, nil).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)
	cc := c.(*client)

	_, err = cc.get("/test", nil)
	assert.Error(t, err)
	assert.IsType(t, &ErrHTTPNotOK{}, err)
}

func TestClient_PostError(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	x.EXPECT().Do(mock.Anything).Return(&http.Response{
		Status:     "Internal Server Error",
		StatusCode: 500,
		Body:       nopCloser{bytes.NewBufferString("error")},
	}, nil).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)
	cc := c.(*client)

	_, err = cc.post("/test", map[string]string{"test": "value"})
	assert.Error(t, err)
	assert.IsType(t, &ErrHTTPNotOK{}, err)
	assert.Equal(t, 500, err.(*ErrHTTPNotOK).StatusCode)
}

func TestClient_PutError(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	x.EXPECT().Do(mock.Anything).Return(&http.Response{
		Status:     "Bad Request",
		StatusCode: 400,
		Body:       nopCloser{bytes.NewBufferString("error")},
	}, nil).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)
	cc := c.(*client)

	_, err = cc.put("/test", map[string]string{"test": "value"})
	assert.Error(t, err)
	assert.IsType(t, &ErrHTTPNotOK{}, err)
	assert.Equal(t, 400, err.(*ErrHTTPNotOK).StatusCode)
}

func TestClient_PatchMethod(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	body := map[string]string{"test": "value"}
	bodyBytes, _ := json.Marshal(body)

	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodPatch, "/test", "", bodyBytes))).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBufferString("success")},
	}, nil).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)
	cc := c.(*client)

	resp, err := cc.patch("/test", body)
	assert.NoError(t, err)
	assert.Equal(t, "success", string(resp))
}

func TestClient_DeleteMethod(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodDelete, "/test", "", nil))).Return(&http.Response{
		Status:     "OK",
		StatusCode: 204,
		Body:       nopCloser{bytes.NewBufferString("")},
	}, nil).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)
	cc := c.(*client)

	resp, err := cc.delete("/test")
	assert.NoError(t, err)
	assert.Equal(t, "", string(resp))
}

func TestClient_FindUserByEmail_MultipleResults(t *testing.T) {
	multipleResults, _ := json.Marshal(&interfaces.UserFilterResults{
		TotalResults: 2,
		Resources: []interfaces.User{
			{Username: "test@example.com"},
			{Username: "test2@example.com"},
		},
	})

	rq := "filter=userName+eq+%22test%40example.com%22"

	x := mocks.NewMockHttpClient(t)
	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodGet, "/Users", rq, nil))).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBuffer(multipleResults)},
	}, nil).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	u, err := c.FindUserByEmail("test@example.com")
	assert.Nil(t, u)
	assert.EqualError(t, err, ErrUserNotFound.Error())
}

func TestClient_FindUserByEmail_UnmarshalError(t *testing.T) {
	invalidJSON := "invalid json"
	rq := "filter=userName+eq+%22test%40example.com%22"

	x := mocks.NewMockHttpClient(t)
	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodGet, "/Users", rq, nil))).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBufferString(invalidJSON)},
	}, nil).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	u, err := c.FindUserByEmail("test@example.com")
	assert.Nil(t, u)
	assert.Error(t, err)
}

func TestClient_FindGroupByDisplayName_MultipleResults(t *testing.T) {
	multipleResults, _ := json.Marshal(&interfaces.GroupFilterResults{
		TotalResults: 2,
		Resources: []interfaces.Group{
			{DisplayName: "testGroup"},
			{DisplayName: "testGroup2"},
		},
	})

	rq := "filter=displayName+eq+%22testGroup%22"

	x := mocks.NewMockHttpClient(t)
	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodGet, "/Groups", rq, nil))).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBuffer(multipleResults)},
	}, nil).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	g, err := c.FindGroupByDisplayName("testGroup")
	assert.Nil(t, g)
	assert.EqualError(t, err, ErrGroupNotFound.Error())
}

func TestClient_FindGroupByDisplayName_UnmarshalError(t *testing.T) {
	invalidJSON := "invalid json"
	rq := "filter=displayName+eq+%22testGroup%22"

	x := mocks.NewMockHttpClient(t)
	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodGet, "/Groups", rq, nil))).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBufferString(invalidJSON)},
	}, nil).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	g, err := c.FindGroupByDisplayName("testGroup")
	assert.Nil(t, g)
	assert.Error(t, err)
}

func TestClient_CreateUser_NilUser(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	u, err := c.CreateUser(nil)
	assert.Nil(t, u)
	assert.EqualError(t, err, ErrUserNotSpecified.Error())
}

func TestClient_CreateUser_UnmarshalError(t *testing.T) {
	nu := NewUser("Lee", "Packham", "test@example.com", true, "google_id")
	requestJSON, _ := json.Marshal(nu)
	invalidJSON := "invalid json"

	x := mocks.NewMockHttpClient(t)
	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodPost, "/Users", "", requestJSON))).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBufferString(invalidJSON)},
	}, nil).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	u, err := c.CreateUser(nu)
	assert.Nil(t, u)
	assert.Error(t, err)
}

func TestClient_CreateUser_NoIDReturned(t *testing.T) {
	nu := NewUser("Lee", "Packham", "test@example.com", true, "google_id")

	// Response without ID
	responseUser := interfaces.User{
		Username:    "test@example.com",
		DisplayName: "Lee Packham",
		Active:      true,
	}
	response, _ := json.Marshal(responseUser)

	// Mock for CreateUser call
	x := mocks.NewMockHttpClient(t)
	x.EXPECT().Do(mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodPost && req.URL.Path == "/Users"
	})).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBuffer(response)},
	}, nil).Once()

	// Mock for FindUserByEmail call (fallback when no ID returned)
	findResult, _ := json.Marshal(&interfaces.UserFilterResults{
		TotalResults: 1,
		Resources: []interfaces.User{
			{
				ID:       "foundUserId",
				Username: "test@example.com",
			},
		},
	})
	x.EXPECT().Do(mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodGet && req.URL.Path == "/Users" &&
			req.URL.RawQuery == "filter=userName+eq+%22test%40example.com%22"
	})).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBuffer(findResult)},
	}, nil).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	u, err := c.CreateUser(nu)
	assert.NotNil(t, u)
	assert.NoError(t, err)
	assert.Equal(t, "foundUserId", u.ID)
}

func TestClient_UpdateUser_NilUser(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	u, err := c.UpdateUser(nil)
	assert.Nil(t, u)
	assert.EqualError(t, err, ErrUserNotFound.Error())
}

func TestClient_UpdateUser_UnmarshalError(t *testing.T) {
	nu := UpdateUser("userId", "Lee", "Packham", "test@example.com", true, "google_id")
	requestJSON, _ := json.Marshal(nu)
	invalidJSON := "invalid json"

	x := mocks.NewMockHttpClient(t)
	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodPut, "/Users/userId", "", requestJSON))).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBufferString(invalidJSON)},
	}, nil).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	u, err := c.UpdateUser(nu)
	assert.Nil(t, u)
	assert.Error(t, err)
}

func TestClient_UpdateUser_NoIDReturned(t *testing.T) {
	nu := UpdateUser("userId", "Lee", "Packham", "test@example.com", true, "google_id")

	// Response without ID
	responseUser := interfaces.User{
		Username:    "test@example.com",
		DisplayName: "Lee Packham",
		Active:      true,
	}
	response, _ := json.Marshal(responseUser)

	// Mock for UpdateUser call
	x := mocks.NewMockHttpClient(t)
	x.EXPECT().Do(mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodPut && req.URL.Path == "/Users/userId"
	})).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBuffer(response)},
	}, nil).Once()

	// Mock for FindUserByEmail call (fallback when no ID returned)
	findResult, _ := json.Marshal(&interfaces.UserFilterResults{
		TotalResults: 1,
		Resources: []interfaces.User{
			{
				ID:       "foundUserId",
				Username: "test@example.com",
			},
		},
	})
	x.EXPECT().Do(mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodGet && req.URL.Path == "/Users" &&
			req.URL.RawQuery == "filter=userName+eq+%22test%40example.com%22"
	})).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBuffer(findResult)},
	}, nil).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	u, err := c.UpdateUser(nu)
	assert.NotNil(t, u)
	assert.NoError(t, err)
	assert.Equal(t, "foundUserId", u.ID)
}

func TestClient_CreateGroup_NilGroup(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	g, err := c.CreateGroup(nil)
	assert.Nil(t, g)
	assert.EqualError(t, err, ErrGroupNotSpecified.Error())
}

func TestClient_CreateGroup_UnmarshalError(t *testing.T) {
	ng := NewGroup("test_group@example.com", "google_id")
	requestJSON, _ := json.Marshal(ng)
	invalidJSON := "invalid json"

	x := mocks.NewMockHttpClient(t)
	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodPost, "/Groups", "", requestJSON))).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBufferString(invalidJSON)},
	}, nil).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	g, err := c.CreateGroup(ng)
	assert.Nil(t, g)
	assert.Error(t, err)
}

func TestClient_CreateGroup_NoIDReturned(t *testing.T) {
	ng := NewGroup("test_group@example.com", "google_id")

	// Response without ID
	responseGroup := interfaces.Group{
		DisplayName: "test_group@example.com",
	}
	response, _ := json.Marshal(responseGroup)

	// Mock for CreateGroup call
	x := mocks.NewMockHttpClient(t)
	x.EXPECT().Do(mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodPost && req.URL.Path == "/Groups"
	})).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBuffer(response)},
	}, nil).Once()

	// Mock for FindGroupByDisplayName call (fallback when no ID returned)
	findResult, _ := json.Marshal(&interfaces.GroupFilterResults{
		TotalResults: 1,
		Resources: []interfaces.Group{
			{
				ID:          "foundGroupId",
				DisplayName: "test_group@example.com",
			},
		},
	})
	x.EXPECT().Do(mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == http.MethodGet && req.URL.Path == "/Groups" &&
			req.URL.RawQuery == "filter=displayName+eq+%22test_group%40example.com%22"
	})).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBuffer(findResult)},
	}, nil).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	g, err := c.CreateGroup(ng)
	assert.NotNil(t, g)
	assert.NoError(t, err)
	assert.Equal(t, "foundGroupId", g.ID)
}

func TestErrHTTPNotOK_Error(t *testing.T) {
	err := &ErrHTTPNotOK{StatusCode: 404}
	assert.Equal(t, "status of http response was 404", err.Error())
}

func TestBeforeSendAddFilter(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/test", nil)

	transformer := beforeSendAddFilter("userName eq \"test@example.com\"")
	transformer(req)

	expected := "filter=userName+eq+%22test%40example.com%22"
	assert.Equal(t, expected, req.URL.RawQuery)
}

func TestClient_GroupChangeOperation_HTTPError(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	x.EXPECT().Do(mock.Anything).Return(nil, errors.New("network error")).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)
	cc := c.(*client)

	g := &interfaces.Group{
		ID:          "groupId",
		DisplayName: "testGroup",
	}

	u := &interfaces.User{
		ID:       "userId",
		Username: "test@example.com",
	}

	err = cc.groupChangeOperation(OperationAdd, u, g)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "network error")
}

func TestClient_ReadBodyError(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	// Create a response with a body that will cause a read error
	errorReader := &errorReader{}
	x.EXPECT().Do(mock.Anything).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       errorReader,
	}, nil).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)
	cc := c.(*client)

	_, err = cc.get("/test", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read error")
}

// Helper type for testing read errors
type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}

func (e *errorReader) Close() error {
	return nil
}

// Additional edge case tests for better coverage

func TestClient_HTTPMethodsWithReadBodyError(t *testing.T) {
	tests := []struct {
		name   string
		method func(*client, string, interface{}) ([]byte, error)
		path   string
		body   interface{}
	}{
		{"POST", (*client).post, "/test", map[string]string{"test": "value"}},
		{"PUT", (*client).put, "/test", map[string]string{"test": "value"}},
		{"PATCH", (*client).patch, "/test", map[string]string{"test": "value"}},
		{"DELETE", func(c *client, path string, _ interface{}) ([]byte, error) { return c.delete(path) }, "/test", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_ReadBodyError", func(t *testing.T) {
			x := mocks.NewMockHttpClient(t)
			errorReader := &errorReader{}
			x.EXPECT().Do(mock.Anything).Return(&http.Response{
				Status:     "OK",
				StatusCode: 200,
				Body:       errorReader,
			}, nil).Once()

			c, err := NewClient(x, &Config{
				Endpoint: baseUrl,
				Token:    "bearerToken",
			})
			require.NoError(t, err)
			cc := c.(*client)

			_, err = tt.method(cc, tt.path, tt.body)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "read error")
		})
	}
}

func TestClient_HTTPMethodsWithNetworkError(t *testing.T) {
	tests := []struct {
		name   string
		method func(*client, string, interface{}) ([]byte, error)
		path   string
		body   interface{}
	}{
		{"GET", func(c *client, path string, _ interface{}) ([]byte, error) { return c.get(path, nil) }, "/test", nil},
		{"POST", (*client).post, "/test", map[string]string{"test": "value"}},
		{"PUT", (*client).put, "/test", map[string]string{"test": "value"}},
		{"PATCH", (*client).patch, "/test", map[string]string{"test": "value"}},
		{"DELETE", func(c *client, path string, _ interface{}) ([]byte, error) { return c.delete(path) }, "/test", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_NetworkError", func(t *testing.T) {
			x := mocks.NewMockHttpClient(t)
			x.EXPECT().Do(mock.Anything).Return(nil, errors.New("network error")).Once()

			c, err := NewClient(x, &Config{
				Endpoint: baseUrl,
				Token:    "bearerToken",
			})
			require.NoError(t, err)
			cc := c.(*client)

			_, err = tt.method(cc, tt.path, tt.body)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "network error")
		})
	}
}

func TestClient_FindUserByEmail_NetworkError(t *testing.T) {
	x := mocks.NewMockHttpClient(t)
	x.EXPECT().Do(mock.Anything).Return(nil, errors.New("network error")).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	u, err := c.FindUserByEmail("test@example.com")
	assert.Nil(t, u)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "network error")
}

func TestClient_FindGroupByDisplayName_NetworkError(t *testing.T) {
	x := mocks.NewMockHttpClient(t)
	x.EXPECT().Do(mock.Anything).Return(nil, errors.New("network error")).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	g, err := c.FindGroupByDisplayName("testGroup")
	assert.Nil(t, g)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "network error")
}

func TestClient_CreateUser_NetworkError(t *testing.T) {
	nu := NewUser("Lee", "Packham", "test@example.com", true, "google_id")
	x := mocks.NewMockHttpClient(t)
	x.EXPECT().Do(mock.Anything).Return(nil, errors.New("network error")).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	u, err := c.CreateUser(nu)
	assert.Nil(t, u)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "network error")
}

func TestClient_UpdateUser_NetworkError(t *testing.T) {
	nu := UpdateUser("userId", "Lee", "Packham", "test@example.com", true, "google_id")
	x := mocks.NewMockHttpClient(t)
	x.EXPECT().Do(mock.Anything).Return(nil, errors.New("network error")).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	u, err := c.UpdateUser(nu)
	assert.Nil(t, u)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "network error")
}

func TestClient_CreateGroup_NetworkError(t *testing.T) {
	ng := NewGroup("test_group@example.com", "google_id")
	x := mocks.NewMockHttpClient(t)
	x.EXPECT().Do(mock.Anything).Return(nil, errors.New("network error")).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	g, err := c.CreateGroup(ng)
	assert.Nil(t, g)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "network error")
}

func TestClient_DeleteUser_NetworkError(t *testing.T) {
	u := &interfaces.User{
		ID:       "userId",
		Username: "test@example.com",
	}
	x := mocks.NewMockHttpClient(t)
	x.EXPECT().Do(mock.Anything).Return(nil, errors.New("network error")).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	err = c.DeleteUser(u)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "network error")
}

func TestClient_DeleteGroup_NetworkError(t *testing.T) {
	g := &interfaces.Group{
		ID:          "groupId",
		DisplayName: "testGroup",
	}
	x := mocks.NewMockHttpClient(t)
	x.EXPECT().Do(mock.Anything).Return(nil, errors.New("network error")).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	err = c.DeleteGroup(g)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "network error")
}

func TestClient_AddUserToGroup_NetworkError(t *testing.T) {
	g := &interfaces.Group{
		ID:          "groupId",
		DisplayName: "testGroup",
	}
	u := &interfaces.User{
		ID:       "userId",
		Username: "test@example.com",
	}
	x := mocks.NewMockHttpClient(t)
	x.EXPECT().Do(mock.Anything).Return(nil, errors.New("network error")).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	err = c.AddUserToGroup(u, g)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "network error")
}

func TestClient_RemoveUserFromGroup_NetworkError(t *testing.T) {
	g := &interfaces.Group{
		ID:          "groupId",
		DisplayName: "testGroup",
	}
	u := &interfaces.User{
		ID:       "userId",
		Username: "test@example.com",
	}
	x := mocks.NewMockHttpClient(t)
	x.EXPECT().Do(mock.Anything).Return(nil, errors.New("network error")).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	err = c.RemoveUserFromGroup(u, g)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "network error")
}

func TestClient_StatusCodeEdgeCases(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
		shouldErr  bool
	}{
		{"Status200", 200, false},
		{"Status201", 201, false},
		{"Status204", 204, false},
		{"Status199", 199, true},
		{"Status205", 205, true},
		{"Status400", 400, true},
		{"Status500", 500, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			x := mocks.NewMockHttpClient(t)
			x.EXPECT().Do(mock.Anything).Return(&http.Response{
				Status:     fmt.Sprintf("Status %d", tc.statusCode),
				StatusCode: tc.statusCode,
				Body:       nopCloser{bytes.NewBufferString("response")},
			}, nil).Once()

			c, err := NewClient(x, &Config{
				Endpoint: baseUrl,
				Token:    "bearerToken",
			})
			require.NoError(t, err)
			cc := c.(*client)

			_, err = cc.get("/test", nil)
			if tc.shouldErr {
				assert.Error(t, err)
				assert.IsType(t, &ErrHTTPNotOK{}, err)
				assert.Equal(t, tc.statusCode, err.(*ErrHTTPNotOK).StatusCode)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_PrepareRequestInvalidURL(t *testing.T) {
	// This test ensures that prepareRequest handles URL construction properly
	x := mocks.NewMockHttpClient(t)

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)
	cc := c.(*client)

	// Test with valid path
	req, err := cc.prepareRequest(http.MethodGet, "/valid/path", nil)
	assert.NoError(t, err)
	assert.Equal(t, baseUrl+"/valid/path", req.URL.String())

	// Test with path containing special characters
	req, err = cc.prepareRequest(http.MethodGet, "/path with spaces", nil)
	assert.NoError(t, err)
	assert.Contains(t, req.URL.String(), "/path%20with%20spaces")
}

// Tests for dry client implementation

func TestNewDryClient(t *testing.T) {
	x := mocks.NewMockHttpClient(t)
	config := &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	}

	client, err := NewDryClient(x, config)
	assert.NoError(t, err)
	assert.NotNil(t, client)

	// Verify it implements the Client interface
	//nolint:staticcheck // QF1011: ignore unnecessary type declaration
	var _ Client = client
}

func TestNewDryClient_Error(t *testing.T) {
	x := mocks.NewMockHttpClient(t)
	config := &Config{
		Endpoint: "invalid-url",
		Token:    "bearerToken",
	}

	client, err := NewDryClient(x, config)
	assert.Error(t, err)
	assert.Nil(t, client)
}

func TestDryClient_CreateUser(t *testing.T) {
	x := mocks.NewMockHttpClient(t)
	client, err := NewDryClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	user := NewUser("John", "Doe", "john@example.com", true, "google_id")

	result, err := client.CreateUser(user)
	assert.NoError(t, err)
	assert.Equal(t, user, result)
}

func TestDryClient_FindGroupByDisplayName(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	// Mock the underlying client call
	falseResult, _ := json.Marshal(&interfaces.GroupFilterResults{
		TotalResults: 0,
	})
	rq := "filter=displayName+eq+%22testGroup%22"
	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodGet, "/Groups", rq, nil))).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBuffer(falseResult)},
	}, nil).Once()

	client, err := NewDryClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	group, err := client.FindGroupByDisplayName("testGroup")
	assert.Error(t, err)
	assert.Equal(t, ErrGroupNotFound, err)
	assert.Nil(t, group)
}

func TestDryClient_FindUserByEmail(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	// Mock the underlying client call
	falseResult, _ := json.Marshal(&interfaces.UserFilterResults{
		TotalResults: 0,
	})
	rq := "filter=userName+eq+%22test%40example.com%22"
	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodGet, "/Users", rq, nil))).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBuffer(falseResult)},
	}, nil).Once()

	client, err := NewDryClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	user, err := client.FindUserByEmail("test@example.com")
	assert.Error(t, err)
	assert.Equal(t, ErrUserNotFound, err)
	assert.Nil(t, user)
}

func TestDryClient_FindUserByEmail_VirtualUser(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	// Mock the underlying client call to return not found
	falseResult, _ := json.Marshal(&interfaces.UserFilterResults{
		TotalResults: 0,
	})
	rq := "filter=userName+eq+%22john%40example.com%22"
	x.EXPECT().Do(mock.MatchedBy(requestMatcher(http.MethodGet, "/Users", rq, nil))).Return(&http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       nopCloser{bytes.NewBuffer(falseResult)},
	}, nil).Once()

	client, err := NewDryClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	// First create a virtual user
	user := NewUser("John", "Doe", "john@example.com", true, "google_id")
	_, err = client.CreateUser(user)
	require.NoError(t, err)

	// Now try to find it
	foundUser, err := client.FindUserByEmail("john@example.com")
	assert.NoError(t, err)
	assert.NotNil(t, foundUser)
	assert.Equal(t, "john@example.com", foundUser.Username)
}

func TestDryClient_UpdateUser(t *testing.T) {
	x := mocks.NewMockHttpClient(t)
	client, err := NewDryClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	user := UpdateUser("userId", "John", "Doe", "john@example.com", true, "google_id")

	result, err := client.UpdateUser(user)
	assert.NoError(t, err)
	assert.Equal(t, user, result)
}

func TestDryClient_AddUserToGroup(t *testing.T) {
	x := mocks.NewMockHttpClient(t)
	client, err := NewDryClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	user := &interfaces.User{ID: "userId", Username: "test@example.com"}
	group := &interfaces.Group{ID: "groupId", DisplayName: "testGroup"}

	err = client.AddUserToGroup(user, group)
	assert.NoError(t, err)
}

func TestDryClient_RemoveUserFromGroup(t *testing.T) {
	x := mocks.NewMockHttpClient(t)
	client, err := NewDryClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	user := &interfaces.User{ID: "userId", Username: "test@example.com"}
	group := &interfaces.Group{ID: "groupId", DisplayName: "testGroup"}

	err = client.RemoveUserFromGroup(user, group)
	assert.NoError(t, err)
}

func TestDryClient_CreateGroup(t *testing.T) {
	x := mocks.NewMockHttpClient(t)
	client, err := NewDryClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	group := NewGroup("testGroup", "google_id")

	result, err := client.CreateGroup(group)
	assert.NoError(t, err)
	assert.Equal(t, group, result)
}

func TestDryClient_DeleteGroup(t *testing.T) {
	x := mocks.NewMockHttpClient(t)
	client, err := NewDryClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	group := &interfaces.Group{ID: "groupId", DisplayName: "testGroup"}

	err = client.DeleteGroup(group)
	assert.NoError(t, err)
}

func TestDryClient_DeleteUser(t *testing.T) {
	x := mocks.NewMockHttpClient(t)
	client, err := NewDryClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	user := &interfaces.User{ID: "userId", Username: "test@example.com"}

	err = client.DeleteUser(user)
	assert.NoError(t, err)
}

// Tests for additional identitystore_dry methods

func TestDryIdentityStore_DeleteGroupMembership(t *testing.T) {
	// Create a mock for the underlying client
	mockClient := &mockIdentityStoreAPI{}
	store := NewDryIdentityStore(mockClient)

	membershipId := "membership-123"
	identityStoreId := "store-123"
	params := &identitystore.DeleteGroupMembershipInput{
		MembershipId:    &membershipId,
		IdentityStoreId: &identityStoreId,
	}

	_, err := store.DeleteGroupMembership(context.Background(), params)
	assert.NoError(t, err)
}

func TestDryIdentityStore_GetGroupMembershipId(t *testing.T) {
	// Create a mock for the underlying client
	mockClient := &mockIdentityStoreAPI{}
	store := NewDryIdentityStore(mockClient)

	groupId := "group-123"
	identityStoreId := "store-123"
	params := &identitystore.GetGroupMembershipIdInput{
		GroupId:         &groupId,
		IdentityStoreId: &identityStoreId,
	}

	_, err := store.GetGroupMembershipId(context.Background(), params)
	assert.NoError(t, err)
}

func TestDryIdentityStore_ListGroupMemberships(t *testing.T) {
	// Create a mock for the underlying client
	mockClient := &mockIdentityStoreAPI{}
	store := NewDryIdentityStore(mockClient)

	groupId := "group-123"
	identityStoreId := "store-123"
	params := &identitystore.ListGroupMembershipsInput{
		GroupId:         &groupId,
		IdentityStoreId: &identityStoreId,
	}

	_, err := store.ListGroupMemberships(context.Background(), params)
	assert.NoError(t, err)
}

func TestDryIdentityStore_ListUsers(t *testing.T) {
	// Create a mock for the underlying client
	mockClient := &mockIdentityStoreAPI{}
	store := NewDryIdentityStore(mockClient)

	identityStoreId := "store-123"
	params := &identitystore.ListUsersInput{
		IdentityStoreId: &identityStoreId,
	}

	_, err := store.ListUsers(context.Background(), params)
	assert.NoError(t, err)
}

func TestDryIdentityStore_CreateUser(t *testing.T) {
	// Create a mock for the underlying client
	mockClient := &mockIdentityStoreAPI{}
	store := NewDryIdentityStore(mockClient)

	userName := "test@example.com"
	identityStoreId := "store-123"
	params := &identitystore.CreateUserInput{
		UserName:        &userName,
		IdentityStoreId: &identityStoreId,
	}

	result, err := store.CreateUser(context.Background(), params)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test@example.com-virtual", *result.UserId)
}

// Mock implementation for IdentityStoreAPI
type mockIdentityStoreAPI struct{}

func (m *mockIdentityStoreAPI) ListGroups(ctx context.Context, params *identitystore.ListGroupsInput, optFns ...func(*identitystore.Options)) (*identitystore.ListGroupsOutput, error) {
	return &identitystore.ListGroupsOutput{}, nil
}

func (m *mockIdentityStoreAPI) ListUsers(ctx context.Context, params *identitystore.ListUsersInput, optFns ...func(*identitystore.Options)) (*identitystore.ListUsersOutput, error) {
	return &identitystore.ListUsersOutput{}, nil
}

func (m *mockIdentityStoreAPI) ListGroupMemberships(ctx context.Context, params *identitystore.ListGroupMembershipsInput, optFns ...func(*identitystore.Options)) (*identitystore.ListGroupMembershipsOutput, error) {
	return &identitystore.ListGroupMembershipsOutput{}, nil
}

func (m *mockIdentityStoreAPI) IsMemberInGroups(ctx context.Context, params *identitystore.IsMemberInGroupsInput, optFns ...func(*identitystore.Options)) (*identitystore.IsMemberInGroupsOutput, error) {
	return &identitystore.IsMemberInGroupsOutput{}, nil
}

func (m *mockIdentityStoreAPI) GetGroupMembershipId(ctx context.Context, params *identitystore.GetGroupMembershipIdInput, optFns ...func(*identitystore.Options)) (*identitystore.GetGroupMembershipIdOutput, error) {
	return &identitystore.GetGroupMembershipIdOutput{}, nil
}

func (m *mockIdentityStoreAPI) DeleteGroupMembership(ctx context.Context, params *identitystore.DeleteGroupMembershipInput, optFns ...func(*identitystore.Options)) (*identitystore.DeleteGroupMembershipOutput, error) {
	return &identitystore.DeleteGroupMembershipOutput{}, nil
}

func (m *mockIdentityStoreAPI) CreateGroup(ctx context.Context, params *identitystore.CreateGroupInput, optFns ...func(*identitystore.Options)) (*identitystore.CreateGroupOutput, error) {
	return &identitystore.CreateGroupOutput{}, nil
}

func (m *mockIdentityStoreAPI) DeleteGroup(ctx context.Context, params *identitystore.DeleteGroupInput, optFns ...func(*identitystore.Options)) (*identitystore.DeleteGroupOutput, error) {
	return &identitystore.DeleteGroupOutput{}, nil
}

func (m *mockIdentityStoreAPI) CreateGroupMembership(ctx context.Context, params *identitystore.CreateGroupMembershipInput, optFns ...func(*identitystore.Options)) (*identitystore.CreateGroupMembershipOutput, error) {
	return &identitystore.CreateGroupMembershipOutput{}, nil
}

func (m *mockIdentityStoreAPI) DeleteUser(ctx context.Context, params *identitystore.DeleteUserInput, optFns ...func(*identitystore.Options)) (*identitystore.DeleteUserOutput, error) {
	return &identitystore.DeleteUserOutput{}, nil
}

func (m *mockIdentityStoreAPI) CreateUser(ctx context.Context, params *identitystore.CreateUserInput, optFns ...func(*identitystore.Options)) (*identitystore.CreateUserOutput, error) {
	return &identitystore.CreateUserOutput{}, nil
}

// Test for close function error handling

func TestCloseWithError(t *testing.T) {
	// Create a mock ReadCloser that returns an error on Close
	errorCloser := &errorCloser{}

	// This should not panic and should log the error
	close(errorCloser)
}

// Helper type for testing close errors
type errorCloser struct{}

func (e *errorCloser) Read(p []byte) (n int, err error) {
	return 0, nil
}

func (e *errorCloser) Close() error {
	return errors.New("close error")
}

// Test for prepareRequest edge cases

func TestClient_PrepareRequestNilBody(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)
	cc := c.(*client)

	// Test with nil body
	req, err := cc.prepareRequest(http.MethodGet, "/test", nil)
	assert.NoError(t, err)
	assert.Equal(t, "Bearer bearerToken", req.Header.Get("Authorization"))
	assert.Equal(t, "application/scim+json", req.Header.Get("Content-Type"))
}

// Test patch and delete methods with different status codes

func TestClient_PatchWithErrorStatusCode(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	x.EXPECT().Do(mock.Anything).Return(&http.Response{
		Status:     "Internal Server Error",
		StatusCode: 500,
		Body:       nopCloser{bytes.NewBufferString("error")},
	}, nil).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)
	cc := c.(*client)

	_, err = cc.patch("/test", map[string]string{"test": "value"})
	assert.Error(t, err)
	assert.IsType(t, &ErrHTTPNotOK{}, err)
	assert.Equal(t, 500, err.(*ErrHTTPNotOK).StatusCode)
}

func TestClient_DeleteWithErrorStatusCode(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	x.EXPECT().Do(mock.Anything).Return(&http.Response{
		Status:     "Not Found",
		StatusCode: 404,
		Body:       nopCloser{bytes.NewBufferString("not found")},
	}, nil).Once()

	c, err := NewClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)
	cc := c.(*client)

	_, err = cc.delete("/test")
	assert.Error(t, err)
	assert.IsType(t, &ErrHTTPNotOK{}, err)
	assert.Equal(t, 404, err.(*ErrHTTPNotOK).StatusCode)
}

func TestDryClient_FindUserByEmail_NetworkError(t *testing.T) {
	x := mocks.NewMockHttpClient(t)

	// Mock the underlying client call to return a network error (not ErrUserNotFound)
	x.EXPECT().Do(mock.Anything).Return(nil, errors.New("network error")).Once()

	client, err := NewDryClient(x, &Config{
		Endpoint: baseUrl,
		Token:    "bearerToken",
	})
	require.NoError(t, err)

	user, err := client.FindUserByEmail("test@example.com")
	assert.Nil(t, user)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "network error")
}
