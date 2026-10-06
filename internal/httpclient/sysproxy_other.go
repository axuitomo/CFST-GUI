//go:build !windows

package httpclient

import (
	"net/http"
	"net/url"
)

// SystemProxy resolves the proxy for a request. On this platform the system
// proxy is only observable through the environment.
func SystemProxy(req *http.Request) (*url.URL, error) {
	return http.ProxyFromEnvironment(req)
}
