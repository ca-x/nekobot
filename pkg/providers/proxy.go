package providers

import (
	"fmt"
	"net/http"
	"net/url"
)

// NewHTTPClientWithProxy creates an http.Client configured with the given proxy URL.
// If proxyURL is empty, returns a default client with no proxy.
func NewHTTPClientWithProxy(proxyURL string) (*http.Client, error) {
	if proxyURL == "" {
		return &http.Client{Timeout: 0}, nil
	}

	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL %q: %w", proxyURL, err)
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(parsed),
	}

	return &http.Client{
		Timeout:   0, // No timeout, handled per-request
		Transport: transport,
	}, nil
}
