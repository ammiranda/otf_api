package otf_api

import (
	"fmt"
	"net/http"
)

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

// AuthMiddleware returns a Middleware that sets the Authorization and
// Content-Type headers dynamically from the Client's current token.
// If a request receives a 401 response and the Client has a refresh
// token, it will attempt to refresh the token and retry the request
// once (only for requests where the body can be re-read).
func AuthMiddleware(c *Client) Middleware {
	return func(rt http.RoundTripper) http.RoundTripper {
		return internalRoundTripper(func(req *http.Request) (*http.Response, error) {
			req.Header.Set("Authorization", "Bearer "+c.Token)
			req.Header.Set("Content-Type", "application/json")

			res, err := rt.RoundTrip(req)
			if err != nil {
				return res, err
			}

			if res.StatusCode == http.StatusUnauthorized && c.RefreshToken != "" {
				if req.Body == nil || req.GetBody != nil {
					res.Body.Close()

					if refreshErr := c.RefreshAuth(req.Context()); refreshErr != nil {
						return nil, fmt.Errorf("token refresh failed: %w", refreshErr)
					}

					newReq := req.Clone(req.Context())
					newReq.Header.Set("Authorization", "Bearer "+c.Token)
					newReq.Header.Set("Content-Type", "application/json")

					return rt.RoundTrip(newReq)
				}
			}

			return res, nil
		})
	}
}
