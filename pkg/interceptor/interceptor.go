// Copied from: https://github.com/golang/go/issues/38479#issuecomment-1962939824
package interceptor

import "net/http"

type Interceptor func(http.RoundTripper) InterceptorRT

type InterceptorRT func(*http.Request) (*http.Response, error)

func (irt InterceptorRT) RoundTrip(req *http.Request) (*http.Response, error) {
	return irt(req)
}

// InterceptorChain is a series of [Interceptor] functions that will be applied
// on each request. A chain should be supplied as the [http.Client.Transport]
// on a http client.
func InterceptorChain(rt http.RoundTripper, interceptors ...Interceptor) http.RoundTripper {
	if rt == nil {
		rt = http.DefaultTransport
	}

	for _, interceptor := range interceptors {
		rt = interceptor(rt)
	}

	return rt
}
