package aws

import "net/http"

// IHttpClient is a generic HTTP Do interface
type IHttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}
