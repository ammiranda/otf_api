package otf_api

import (
	"fmt"
	"log"
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
// Content-Type headers dynamically from the Client's current token. If
// the token is empty or expired (see Client.NeedAuth), it refreshes
// the token before the request is sent, falling back to
// Client.FallbackAuth when the refresh token is unavailable or the
// refresh fails. If a request still receives a 401 response, it will
// attempt to refresh the token (with the same fallback) and retry the
// request once (only for requests where the body can be re-read).
func AuthMiddleware(c *Client) Middleware {
	return func(rt http.RoundTripper) http.RoundTripper {
		return internalRoundTripper(func(req *http.Request) (*http.Response, error) {
			if c.NeedAuth() && (c.HasRefreshToken() || c.FallbackAuth != nil) {
				if authErr := c.refreshAuthOrFallback(req.Context()); authErr != nil {
					return nil, fmt.Errorf("auth refresh failed: %w", authErr)
				}
			}

			req.Header.Set("Authorization", "Bearer "+c.TokenValue())
			req.Header.Set("Content-Type", "application/json")

			res, err := rt.RoundTrip(req)
			if err != nil {
				return res, err
			}

			if res.StatusCode == http.StatusUnauthorized && (c.HasRefreshToken() || c.FallbackAuth != nil) {
				if req.Body == nil || req.GetBody != nil {
					if err := res.Body.Close(); err != nil {
						log.Printf("error closing response body: %v", err)
					}

					if authErr := c.refreshAuthOrFallback(req.Context()); authErr != nil {
						return nil, fmt.Errorf("auth refresh failed: %w", authErr)
					}

					newReq := req.Clone(req.Context())
					newReq.Header.Set("Authorization", "Bearer "+c.TokenValue())
					newReq.Header.Set("Content-Type", "application/json")

					return rt.RoundTrip(newReq)
				}
			}

			return res, nil
		})
	}
}
