package otf_api

type BookingRequest struct {
	Confirmed bool   `json:"confirmed"`
	ClassUUID string `json:"classUUId"`
	Waitlist  bool   `json:"waitlist"`
}

// func (c *Client) BookClass(
// 	ctx context.Context,
// 	classID string,
// 	waitlist bool,
// ) error {

// }
