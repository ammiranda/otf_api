package otf_api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	StudioIDsQueryParamKey = "studio_ids"
)

type StudioClassStudioAddress struct {
	Line1      string `json:"line1"`
	City       string `json:"city"`
	State      string `json:"state"`
	Country    string `json:"country"`
	PostalCode string `json:"postal_code"`
}

type StudioClassStudio struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	PhoneNumber string                   `json:"phone_number"`
	Latitude    float64                  `json:"latitude"`
	Longitude   float64                  `json:"longitude"`
	Address     StudioClassStudioAddress `json:"address"`
}

type StudioClass struct {
	ID                string            `json:"id"`
	StartsAt          time.Time         `json:"starts_at"`
	EndsAt            time.Time         `json:"ends_at"`
	Name              string            `json:"name"`
	MaxCapacity       int               `json:"max_capacity"`
	BookingCapacity   int               `json:"booking_capacity"`
	WaitlistSize      int               `json:"waitlist_size"`
	WaitlistAvailable bool              `json:"waitlist_available"`
	Canceled          bool              `json:"canceled"`
	Studio            StudioClassStudio `json:"studio"`
}

type StudioScheduleResponse struct {
	Items []StudioClass `json:"items"`
}

type FilterValues struct {
	Value       string `json:"value"`
	DisplayName string `json:"display_name"`
	IconURL     string `json:"icon_url"`
}

type FilterItem struct {
	Name           string         `json:"name"`
	DisplayName    string         `json:"display_name"`
	ClassFieldName string         `json:"class_field_type"`
	Values         []FilterValues `json:"values"`
}

type ClassTypeFiltersResponse struct {
	Items []FilterItem
}

// GetStudiosSchedules
func (c *Client) GetStudiosSchedules(
	ctx context.Context,
	studioIDs []string,
) (StudioScheduleResponse, error) {
	params := url.Values{
		StudioIDsQueryParamKey: studioIDs,
	}

	url := c.BaseIOURL + "classes?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return StudioScheduleResponse{}, err
	}

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return StudioScheduleResponse{}, err
	}
	defer res.Body.Close()

	parsedResp := StudioScheduleResponse{}
	err = json.NewDecoder(res.Body).Decode(&parsedResp)
	if err != nil {
		return StudioScheduleResponse{}, fmt.Errorf("error parsing response: %w", err)
	}

	return parsedResp, nil
}

func (c *Client) GetClassTypeFilter(
	ctx context.Context,
) (ClassTypeFiltersResponse, error) {
	url := c.BaseIOURL + "classes/filters"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ClassTypeFiltersResponse{}, err
	}

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return ClassTypeFiltersResponse{}, err
	}
	defer res.Body.Close()

	parsedResp := ClassTypeFiltersResponse{}
	err = json.NewDecoder(res.Body).Decode(&parsedResp)
	if err != nil {
		return ClassTypeFiltersResponse{}, err
	}

	return parsedResp, nil
}
