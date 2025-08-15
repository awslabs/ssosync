package http

import (
	"net/http"
	"testing"
)

func TestClientInterface(t *testing.T) {
	// Test that http.Client implements our Client interface
	var client Client = &http.Client{}

	// Test that we can call the interface method
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	_, err := client.Do(req)

	// We expect an error since we're not making a real request
	if err == nil {
		t.Log("Client interface implemented correctly")
	}
}

// MockClient is a simple mock implementation for testing
type MockClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockClient) Do(req *http.Request) (*http.Response, error) {
	if m.DoFunc != nil {
		return m.DoFunc(req)
	}
	return &http.Response{StatusCode: 200}, nil
}

func TestMockClient(t *testing.T) {
	mock := &MockClient{}

	// Test default behavior
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp, err := mock.Do(req)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", resp.StatusCode)
	}
}

func TestMockClientWithCustomFunc(t *testing.T) {
	mock := &MockClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 404}, nil
		},
	}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp, err := mock.Do(req)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if resp.StatusCode != 404 {
		t.Errorf("Expected status code 404, got %d", resp.StatusCode)
	}
}
