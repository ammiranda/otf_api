package otf_api

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type BookingRequest struct {
	ID                string `json:"id"`
	PayingStudioID    string `json:"paying_studio_id"`
	PersonID          string `json:"person_id"`
	MemberID          string `json:"member_id"`
	ServiceName       string `json:"service_name"`
	CheckedIn         bool   `json:"checked_in"`
	CrossRegional     bool   `json:"cross_regional"`
	LateCanceled      bool   `json:"late_canceled"`
	Intro             bool   `json:"intro"`
	MboBookingID      string `json:"mbo_booking_id"`
	MboUniqueID       string `json:"mbo_unique_id"`
	MboPayingUniqueID string `json:"mbo_paying_unique_id"`
	Canceled          bool   `json:"canceled"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
	Ratable           bool   `json:"ratable"`
	Class             Class  `json:"class"`
}

type Class struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	StartsAtLocal string `json:"starts_at_local"`
	StartsAt      string `json:"starts_at"`
	Studio        BookingStudio `json:"studio"`
	Coach         Coach  `json:"coach"`
}

type BookingStudio struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	MboStudioID  string  `json:"mbo_studio_id"`
	TimeZone     string  `json:"time_zone"`
	Email        string  `json:"email"`
	Address      Address `json:"address"`
	CurrencyCode string  `json:"currency_code"`
	PhoneNumber  string  `json:"phone_number"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
}

type Address struct {
	Line1      string `json:"line1"`
	City       string `json:"city"`
	State      string `json:"state"`
	Country    string `json:"country"`
	PostalCode string `json:"postal_code"`
}

type Coach struct {
	FirstName string `json:"first_name"`
	ImageURL  string `json:"image_url"`
}

type BookingResponse struct {
	Items   []BookingRequest `json:"items,omitempty"`
	Booking *BookingRequest  `json:"booking,omitempty"`
}

// BookClass attempts to book a class using the OTF API.
// Returns an error if the booking fails.
func (c *Client) BookClass(
	ctx context.Context,
	bookingReq interface{},
) error {
	jsonBody, err := json.Marshal(bookingReq)
	if err != nil {
		return fmt.Errorf("failed marshaling request body: %w", err)
	}

	// Debug: log the request body
	log.Printf("Booking request body: %s", string(jsonBody))

	apiURL, err := url.JoinPath(c.BaseIOURL, "bookings/me")
	if err != nil {
		return fmt.Errorf("error joining path: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("error preparing request: %w", err)
	}

	// Set required headers based on README and Charles Proxy capture
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", c.Token)
	req.Header.Set("otf-locale", "en_US")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("User-Agent", "Orangetheory/403 CFNetwork/3826.600.41 Darwin/24.6.0")
	
	// Add tracing headers that might be required
	req.Header.Set("tracestate", "1891601@nr=0-2-1891601-162062094-9ba640a524edf6ae---1756927588442")
	req.Header.Set("newrelic", "ewoiZCI6IHsKImFjIjogIjE4OTE2MDEiLAoiYXAiOiAiMTYyMDYyMDk0IiwKImlkIjogIjliYTY0MGE1MjRlZGY2YWUiLAoidGkiOiAxNzU2OTI3NTg4NDQyLAoidHIiOiAiNGYwYzI5MDc4NjA3NjAxODc4NGI3NTIxNTE5NzNiMDYiLAoidHkiOiAiTW9iaWxlIgp9LAoidiI6IFsKMCwKMgpdCn0=")
	req.Header.Set("traceparent", "00-4f0c290786076018784b752151973b06-9ba640a524edf6ae-01")

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("error executing request: %w", err)
	}
	defer func() {
		if closeErr := res.Body.Close(); closeErr != nil {
			if err == nil {
				err = fmt.Errorf("error closing response body: %w", closeErr)
			} else {
				log.Printf("Failed to close response body (original error: %v): %v", err, closeErr)
			}
		}
	}()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		var reader io.Reader = res.Body
		
		// Check if response is gzipped
		if strings.Contains(res.Header.Get("Content-Encoding"), "gzip") {
			gzipReader, err := gzip.NewReader(res.Body)
			if err != nil {
				return fmt.Errorf("booking request failed with status code: %d, failed to create gzip reader: %w", res.StatusCode, err)
			}
			defer gzipReader.Close()
			reader = gzipReader
		}
		
		body, err := io.ReadAll(reader)
		if err != nil {
			return fmt.Errorf("booking request failed with status code: %d, and failed to read response body: %w", res.StatusCode, err)
		}
		
		return fmt.Errorf("booking request failed with status code: %d, response body: %s", res.StatusCode, string(body))
	}

	return nil
}

// CancelBooking cancels a booking by ID
func (c *Client) CancelBooking(
	ctx context.Context,
	bookingID string,
) error {
	apiURL, err := url.JoinPath(c.BaseIOURL, "bookings/me", bookingID)
	if err != nil {
		return fmt.Errorf("error joining path: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, apiURL, nil)
	if err != nil {
		return fmt.Errorf("error preparing request: %w", err)
	}

	// Set required headers
	req.Header.Set("Authorization", c.Token)
	req.Header.Set("otf-locale", "en_US")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("User-Agent", "Orangetheory/403 CFNetwork/3826.600.41 Darwin/24.6.0")

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("error executing request: %w", err)
	}
	defer func() {
		if closeErr := res.Body.Close(); closeErr != nil {
			log.Printf("Failed to close response body: %v", closeErr)
		}
	}()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("cancel request failed with status code: %d, and failed to read response body: %w", res.StatusCode, err)
		}
		return fmt.Errorf("cancel request failed with status code: %d, response body: %s", res.StatusCode, string(body))
	}

	return nil
}

// GetBookings retrieves bookings within a date range
func (c *Client) GetBookings(
	ctx context.Context,
	startsAfter time.Time,
	endsBefore time.Time,
	includeCanceled bool,
) ([]BookingRequest, error) {
	apiURL, err := url.JoinPath(c.BaseIOURL, "bookings/me")
	if err != nil {
		return nil, fmt.Errorf("error joining path: %w", err)
	}

	// Build query parameters
	params := url.Values{}
	params.Set("starts_after", startsAfter.Format(time.RFC3339))
	params.Set("ends_before", endsBefore.Format(time.RFC3339))
	params.Set("include_canceled", fmt.Sprintf("%t", includeCanceled))

	fullURL := fmt.Sprintf("%s?%s", apiURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error preparing request: %w", err)
	}

	// Set required headers
	req.Header.Set("Authorization", c.Token)
	req.Header.Set("otf-locale", "en_US")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("User-Agent", "Orangetheory/403 CFNetwork/3826.600.41 Darwin/24.6.0")

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing request: %w", err)
	}
	defer func() {
		if closeErr := res.Body.Close(); closeErr != nil {
			log.Printf("Failed to close response body: %v", closeErr)
		}
	}()

	if res.StatusCode != http.StatusOK {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, fmt.Errorf("get bookings request failed with status code: %d, and failed to read response body: %w", res.StatusCode, err)
		}
		return nil, fmt.Errorf("get bookings request failed with status code: %d, response body: %s", res.StatusCode, string(body))
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	// Debug: log the response body
	log.Printf("GetBookings response: %s", string(body))

	var response BookingResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %w", err)
	}

	return response.Items, nil
}
