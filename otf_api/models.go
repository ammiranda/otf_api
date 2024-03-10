package otf_api

import (
	"net/http"
	"time"
)

type Client struct {
	BaseIOURL  string
	BaseCOURL  string
	AuthURL    string
	Token      string
	HTTPClient *http.Client
}

type Credentials struct {
	Username string `json:"USERNAME"`
	Password string `json:"PASSWORD"`
}

type AuthenticateRequest struct {
	AuthParameters Credentials `json:"AuthParameters"`
	AuthFlow       string      `json:"AuthFlow"`
	ClientID       string      `json:"ClientId"`
}

type IDToken struct {
	IDToken string `json:"IdToken"`
}

type AuthenticateResponse struct {
	AuthenticationResult IDToken `json:"AuthenticationResult"`
}

type ListStudiosResponse struct {
}

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
