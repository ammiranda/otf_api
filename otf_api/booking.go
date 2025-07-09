package otf_api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type BookingRequest struct {
	Confirmed bool   `json:"confirmed"`
	ClassUUID string `json:"classUUId"`
	Waitlist  bool   `json:"waitlist"`
}

// BookClass attempts to book a class or add to waitlist if the class is full.
// Returns an error if the booking fails.
func (c *Client) BookClass(
	ctx context.Context,
	classID string,
	waitlist bool,
) error {
	reqBody := BookingRequest{
		Confirmed: true,
		ClassUUID: classID,
		Waitlist:  waitlist,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed marshaling request body: %w", err)
	}

	url := c.BaseIOURL + "classes/book"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("error preparing request: %w", err)
	}

	// Set required headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))

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

	if res.StatusCode != http.StatusOK {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("booking request failed with status code: %d, and failed to read response body: %w", res.StatusCode, err)
		}
		return fmt.Errorf("booking request failed with status code: %d, response body: %s", res.StatusCode, string(body))
	}

	return nil
}
