package otf_api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const (
	LatitudeQueryParamKey  = "latitude"
	LongitudeQueryParamKey = "longitude"
	DistanceQueryParamKey  = "distance"
)

type StudioLocation struct {
	PhysicalAddressOne string  `json:"physicalAddress"`
	PhysicalAddressTwo string  `json:"physicalAddress2"`
	PhysicalCity       string  `json:"physicalCity"`
	PhysicalState      string  `json:"physicalState"`
	PhysicalCountry    string  `json:"physicalCountry"`
	Latitude           float64 `json:"latitude"`
	Longitude          float64 `json:"longitude"`
	PhoneNumber        string  `json:"phoneNumber"`
}

type Studio struct {
	StudioUUID     string         `json:"studioUUId"`
	StudioName     string         `json:"studioName"`
	StudioLocation StudioLocation `json:"studioLocation"`
	Distance       float64        `json:"distance"`
}

type ListStudiosRequest struct {
	Latitude  float64 `validate:""`
	Longitude float64 `validate:""`
	Distance  float64 `validate:""`
}

// type ListStudiosResponse struct {
// 	Data
// 	Studios []Studio `json:""`
// }

// ListStudios returns studios that lie within the radius distance (in miles)
// from the lat/long point specified.
func (c *Client) ListStudios(
	ctx context.Context,
	lat float64,
	long float64,
	distance float64,
) error {
	params := url.Values{
		LatitudeQueryParamKey: {
			"30.259373217464326",
		},
		LongitudeQueryParamKey: {
			"-97.70429793893",
		},
		DistanceQueryParamKey: {
			"50.0",
		},
	}
	// params.Add("latitude", "30.259373217464326")
	// params.Add("longitude", "-97.70429793893")
	// params.Add("distance", "30.0")

	url := c.BaseCOURL + "studios?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	req.Header = http.Header{
		"Content-Type": {
			"application/json",
		},
		"Authorization": {
			c.Token,
		},
	}

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	fmt.Println(string(body))

	return nil
}
