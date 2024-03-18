package otf_api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
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
	Latitude  float64 `validate:"required"`
	Longitude float64 `validate:"required"`
	Distance  float64 `validate:"required,gt=0"`
}

type Studios struct {
	Data       []Studio   `json:"studios"`
	Pagination Pagination `json:"pagination"`
}

type Pagination struct {
	PageIndex  int `json:"pageIndex"`
	PageSize   int `json:"pageSize"`
	TotalCount int `json:"totalCount"`
	TotalPages int `json:"totalPages"`
}

type ListStudiosResponse struct {
	Data Studios `json:"data"`
}

// ListStudios returns studios that lie within the radius distance (in miles)
// from the lat/long point specified.
func (c *Client) ListStudios(
	ctx context.Context,
	lat float64,
	long float64,
	distance float64,
) (ListStudiosResponse, error) {
	params := url.Values{
		LatitudeQueryParamKey: {
			toString(lat),
		},
		LongitudeQueryParamKey: {
			toString(long),
		},
		DistanceQueryParamKey: {
			toString(distance),
		},
	}

	u := c.BaseCOURL + "studios?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return ListStudiosResponse{}, err
	}

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return ListStudiosResponse{}, err
	}
	defer res.Body.Close()

	parsedResp := ListStudiosResponse{}
	err = json.NewDecoder(res.Body).Decode(&parsedResp)
	if err != nil {
		return ListStudiosResponse{}, err
	}

	return parsedResp, nil
}

func toString(v float64) string {
	return strconv.FormatFloat(v, 'f', 15, 64)
}
