package otf_api

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	c := NewClient()
	require.NotNil(t, c)
	assert.NotEmpty(t, c.BaseIOURL)
	assert.NotEmpty(t, c.BaseCOURL)
	assert.NotEmpty(t, c.AuthURL)
	assert.NotNil(t, c.HTTPClient)
	assert.NotNil(t, c.HTTPClient.Transport)
	assert.NotNil(t, c.authenticator)
}

func TestToString(t *testing.T) {
	assert.Equal(t, "1.500000000000000", toString(1.5))
	assert.Equal(t, "0.000000000000000", toString(0.0))
	assert.Equal(t, "-123.456000000000003", toString(-123.456))
}

type clientTestSuite struct {
	server *httptest.Server
}

func (s *clientTestSuite) setup(handler http.HandlerFunc) {
	s.server = httptest.NewServer(http.HandlerFunc(handler))
}

func (s *clientTestSuite) teardown() {
	if s.server != nil {
		s.server.Close()
	}
}

func (s *clientTestSuite) client() *Client {
	c := NewClient()
	c.BaseIOURL = s.server.URL + "/"
	c.BaseCOURL = s.server.URL + "/"
	c.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	return c
}

func TestBookClass(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       []byte
		gzip       bool
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "success created",
			statusCode: http.StatusCreated,
			wantErr:    false,
		},
		{
			name:       "success ok",
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "error response",
			statusCode: http.StatusBadRequest,
			body:       []byte(`{"error":"bad request"}`),
			wantErr:    true,
			errMsg:     "booking request failed",
		},
		{
			name:       "gzip error response",
			statusCode: http.StatusBadRequest,
			body:       []byte(`{"error":"bad request"}`),
			gzip:       true,
			wantErr:    true,
			errMsg:     "booking request failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &clientTestSuite{}
			s.setup(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Contains(t, r.URL.Path, "/bookings/me")
				assert.Equal(t, "application/json", r.Header.Get("Accept"))
				if tt.gzip {
					w.Header().Set("Content-Encoding", "gzip")
					gz := gzip.NewWriter(w)
					w.WriteHeader(tt.statusCode)
					gz.Write(tt.body)
					gz.Close()
					return
				}
				w.WriteHeader(tt.statusCode)
				if tt.body != nil {
					w.Write(tt.body)
				}
			})
			defer s.teardown()

			c := s.client()
			err := c.BookClass(context.Background(), CreateBookingRequest{ClassID: "class-1"})
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestCancelBooking(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       []byte
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "success no content",
			statusCode: http.StatusNoContent,
			wantErr:    false,
		},
		{
			name:       "success ok",
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "error response",
			statusCode: http.StatusNotFound,
			body:       []byte(`{"error":"not found"}`),
			wantErr:    true,
			errMsg:     "cancel request failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &clientTestSuite{}
			s.setup(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodDelete, r.Method)
				assert.Contains(t, r.URL.Path, "/bookings/me/booking-1")
				w.WriteHeader(tt.statusCode)
				if tt.body != nil {
					w.Write(tt.body)
				}
			})
			defer s.teardown()

			c := s.client()
			err := c.CancelBooking(context.Background(), "booking-1")
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestGetBookings(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       []byte
		want       []BookingRequest
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			body: mustJSON(BookingResponse{
				Items: []BookingRequest{
					{ID: "booking-1", ServiceName: "Orange 60"},
					{ID: "booking-2", ServiceName: "Orange 3G"},
				},
			}),
			want: []BookingRequest{
				{ID: "booking-1", ServiceName: "Orange 60"},
				{ID: "booking-2", ServiceName: "Orange 3G"},
			},
		},
		{
			name:       "empty response",
			statusCode: http.StatusOK,
			body:       []byte(`{"items":[]}`),
			want:       []BookingRequest{},
		},
		{
			name:       "error response",
			statusCode: http.StatusInternalServerError,
			body:       []byte(`{"error":"server error"}`),
			wantErr:    true,
			errMsg:     "get bookings request failed",
		},
		{
			name:       "invalid json",
			statusCode: http.StatusOK,
			body:       []byte(`not json`),
			wantErr:    true,
			errMsg:     "unmarshaling",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &clientTestSuite{}
			s.setup(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Contains(t, r.URL.Path, "/bookings/me")
				w.WriteHeader(tt.statusCode)
				if tt.body != nil {
					w.Write(tt.body)
				}
			})
			defer s.teardown()

			c := s.client()
			got, err := c.GetBookings(context.Background(), time.Now(), time.Now().AddDate(0, 0, 60), true)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetStudiosSchedules(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       []byte
		want       StudioScheduleResponse
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			body: mustJSON(StudioScheduleResponse{
				Items: []StudioClass{
					{ID: "class-1", Name: "Orange 60"},
				},
			}),
			want: StudioScheduleResponse{
				Items: []StudioClass{
					{ID: "class-1", Name: "Orange 60"},
				},
			},
		},
		{
			name:       "invalid json",
			statusCode: http.StatusOK,
			body:       []byte(`not json`),
			wantErr:    true,
			errMsg:     "error parsing response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &clientTestSuite{}
			s.setup(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Contains(t, r.URL.Path, "/classes")
				assert.Equal(t, []string{"studio-1"}, r.URL.Query()[StudioIDsQueryParamKey])
				w.WriteHeader(tt.statusCode)
				if tt.body != nil {
					w.Write(tt.body)
				}
			})
			defer s.teardown()

			c := s.client()
			got, err := c.GetStudiosSchedules(context.Background(), []string{"studio-1"})
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetClassTypeFilter(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       []byte
		want       ClassTypeFiltersResponse
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			body: mustJSON(ClassTypeFiltersResponse{
				Items: []FilterItem{
					{Name: "class_type", DisplayName: "Class Type"},
				},
			}),
			want: ClassTypeFiltersResponse{
				Items: []FilterItem{
					{Name: "class_type", DisplayName: "Class Type"},
				},
			},
		},
		{
			name:       "invalid json",
			statusCode: http.StatusOK,
			body:       []byte(`not json`),
			wantErr:    true,
			errMsg:     "parsing response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &clientTestSuite{}
			s.setup(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Contains(t, r.URL.Path, "/classes/filters")
				w.WriteHeader(tt.statusCode)
				if tt.body != nil {
					w.Write(tt.body)
				}
			})
			defer s.teardown()

			c := s.client()
			got, err := c.GetClassTypeFilter(context.Background())
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestListStudios(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       []byte
		want       ListStudiosResponse
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "success",
			statusCode: http.StatusOK,
			body: mustJSON(ListStudiosResponse{
				Data: Studios{
					Data: []Studio{
						{StudioUUID: "studio-1", StudioName: "OTF Downtown"},
					},
				},
			}),
			want: ListStudiosResponse{
				Data: Studios{
					Data: []Studio{
						{StudioUUID: "studio-1", StudioName: "OTF Downtown"},
					},
				},
			},
		},
		{
			name:       "invalid json",
			statusCode: http.StatusOK,
			body:       []byte(`not json`),
			wantErr:    true,
			errMsg:     "invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &clientTestSuite{}
			s.setup(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Contains(t, r.URL.Path, "/studios")
				assert.Equal(t, "40.000000000000000", r.URL.Query().Get(LatitudeQueryParamKey))
				assert.Equal(t, "-74.000000000000000", r.URL.Query().Get(LongitudeQueryParamKey))
				assert.Equal(t, "10.000000000000000", r.URL.Query().Get(DistanceQueryParamKey))
				w.WriteHeader(tt.statusCode)
				if tt.body != nil {
					w.Write(tt.body)
				}
			})
			defer s.teardown()

			c := s.client()
			got, err := c.ListStudios(context.Background(), 40.0, -74.0, 10.0)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("marshal failed: %v", err))
	}
	return b
}
