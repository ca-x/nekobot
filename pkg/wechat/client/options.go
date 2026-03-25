package client

import "net/http"

// HTTPDoer is the interface for making HTTP requests.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// ClientOption configures the Client.
type ClientOption func(*Client)

// WithBaseURL sets a custom base URL for the iLink API.
func WithBaseURL(rawURL string) ClientOption {
	return func(c *Client) {
		c.baseURL = rawURL
	}
}

// WithHTTPDoer sets a custom HTTP client.
func WithHTTPDoer(doer HTTPDoer) ClientOption {
	return func(c *Client) {
		c.httpDoer = doer
	}
}
