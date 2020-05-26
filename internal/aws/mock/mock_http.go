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

package mock

import (
	"net/http"
	"reflect"

	"github.com/golang/mock/gomock"
)

// MockIHttpClient is a mock of IHttpClient interface
type MockIHttpClient struct {
	ctrl     *gomock.Controller
	recorder *MockIHttpClientMockRecorder
}

// MockIHttpClientMockRecorder is the mock recorder for MockIHttpClient
type MockIHttpClientMockRecorder struct {
	mock *MockIHttpClient
}

// NewMockIHttpClient creates a new mock instance
func NewMockIHttpClient(ctrl *gomock.Controller) *MockIHttpClient {
	mock := &MockIHttpClient{ctrl: ctrl}
	mock.recorder = &MockIHttpClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockIHttpClient) EXPECT() *MockIHttpClientMockRecorder {
	return m.recorder
}

// Do mocks base method
func (m *MockIHttpClient) Do(req *http.Request) (*http.Response, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Do", req)
	ret0, _ := ret[0].(*http.Response)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Do indicates an expected call of Do
func (mr *MockIHttpClientMockRecorder) Do(req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Do", reflect.TypeOf((*MockIHttpClient)(nil).Do), req)
}
