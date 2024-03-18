package otf_api

import "net/http"

type internalRoundTripper func(*http.Request) (*http.Response, error)

func (rt internalRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return rt(req)
}

type Middleware func(http.RoundTripper) http.RoundTripper

func Chain(rt http.RoundTripper, middlewares ...Middleware) http.RoundTripper {
	if rt == nil {
		rt = http.DefaultTransport
	}

	for _, m := range middlewares {
		rt = m(rt)
	}

	return rt
}

func AddHeader(key string, value string) Middleware {
	return func(rt http.RoundTripper) http.RoundTripper {
		return internalRoundTripper(func(req *http.Request) (*http.Response, error) {
			header := req.Header

			if header == nil {
				header = make(http.Header)
			}

			header.Set(key, value)

			return rt.RoundTrip(req)
		})
	}
}
