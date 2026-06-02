# OTF API Agent Documentation

## Overview

The `otf_api` Go package provides a robust, modular agent for interacting with the OrangeTheory Fitness API suite. This library is designed to simplify authentication, access to studio information, class schedules, and class booking, providing an ergonomic client for developers and a clear extension point for contributors.

## Features
- Authentication against OTF's Cognito-backed Auth service
- Studio search and listing within a radius
- Fetching class schedules for one or many studios
- Booking classes, retrieving booking status, and managing booking life cycle

## Environment Configuration

Requires a `.env` file (at project root) or equivalent shell environment variables with the following keys:
- **OTF_API_IO_BASE_URL** – The Base URL for IO APIs
- **OTF_API_CO_BASE_URL** – The Base URL for CO APIs
- **OTF_AUTH_URL** – The authentication endpoint
- **OTF_CLIENT_ID** – Cognito client ID for authentication

Example `.env`:
```
OTF_API_IO_BASE_URL=https://api.example.com/io/
OTF_API_CO_BASE_URL=https://api.example.com/co/
OTF_AUTH_URL=https://auth.example.com/
OTF_CLIENT_ID=your-otf-client-id
```

## Client Usage (Developers)

### Creating a Client
```go
client, err := otf_api.NewClient()
if err != nil {
    log.Fatal(err)
}
```

### Authentication
```go
err = client.Authenticate(ctx, "your_username", "your_password")
if err != nil {
    log.Fatalf("auth failed: %v", err)
}
```

### List Studios
```go
resp, err := client.ListStudios(ctx, latitude, longitude, distance)
```

### Get Class Schedules
```go
schedule, err := client.GetStudiosSchedules(ctx, []string{"studio_id1", "studio_id2"})
```

### Book a Class
```go
booking := otf_api.CreateBookingRequest{
    ClassID:   "class_id",
    Confirmed: true,
    Waitlist:  false,
}
err = client.BookClass(ctx, booking)
```

## API Methods and Structs

### `type Client struct { ... }`
- Holds Endpoints, Auth Token, http.Client, and MemberID.

### Methods

- `NewClient() (*Client, error)` – Constructor, loads env vars
- `(*Client) Authenticate(ctx, username, password) error` – Authenticates and saves the JWT
- `(*Client) ListStudios(ctx, lat, long, distance) (ListStudiosResponse, error)` – Studio search
- `(*Client) GetStudiosSchedules(ctx, studioIDs) (StudioScheduleResponse, error)` – Class schedules
- `(*Client) BookClass(ctx, CreateBookingRequest) error` – Book class

(See GoDoc for full struct and response details)

## Contribution (Contributors)

- All HTTP calls are routed through the `Client` and its `HTTPClient`, with middleware for auth headers (`middleware.go`).
- Add new endpoints as new `Client` methods. Each method should:
    1. Receive a `context.Context` for cancel/timeout.
    2. Marshal all input data as required by API.
    3. Use appropriate base URL from environment.
    4. Add any needed headers or middlewares using `AddHeader()` and `Chain()`.
    5. Return clear, typed responses and errors.
- Extensions (additional endpoints, new features) should be added as new files/modules if API surface grows significantly.

### Code Standards
- Group related types and actions in dedicated files (`booking.go`, `schedule.go`, etc.).
- Use built-in Go error and context handling for all client methods.
- Write table-driven tests for all functional methods (not yet provided, please contribute).
- Document all exported types and methods with GoDoc comments.

## Testing

- Be sure to stub/mock outbound HTTP calls for unit tests.
- No test suite currently included—contributions welcome for tests covering all endpoints and error scenarios.

## Contact & Help

- For issues, open a GitHub ticket or contact the primary maintainer.
- See the OTF API’s official documentation (internal/external as required) for deeper backend reference.

## References
- [GoDoc: github.com/joho/godotenv](https://pkg.go.dev/github.com/joho/godotenv)
- [OrangeTheory Fitness (Placeholder) API docs]

---
*This document is auto-generated. Please keep up to date as you add new endpoints or logic.*
