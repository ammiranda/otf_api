# OTF API Agent Guide

## Overview

Go module `github.com/ammiranda/otf_api` — a reverse-engineered SDK, CLI, and MCP server for the OrangeTheory Fitness API.

### Components

| Component | Module | Path |
|-----------|--------|------|
| **SDK** | `github.com/ammiranda/otf_api` | `./otf_api/` |
| **CLI** | `cmd/otf-cli` | `./cmd/otf-cli/` |
| **MCP** | `cmd/otf-mcp` | `./cmd/otf-mcp/` |

### Workspace
Uses Go workspaces (`go.work`). All three modules are in the workspace.

## Build

```bash
make build       # both binaries -> bin/otf-cli + bin/otf-mcp
make build-cli   # CLI only
make build-mcp   # MCP only
make test        # go test -v ./...
make lint        # golangci-lint run
```

## SDK (`otf_api`)

### Client creation
```go
client := otf_api.NewClient()
```
Defaults to hardcoded OTF API URLs and iOS Cognito client ID. Override via env vars:
`OTF_API_IO_BASE_URL`, `OTF_API_CO_BASE_URL`, `OTF_AUTH_URL`, `OTF_CLIENT_ID`.

### Authentication
Via AWS Cognito `USER_PASSWORD_AUTH`/`REFRESH_TOKEN_AUTH` flows.
```go
client.Authenticate(ctx, username, password)
client.SetToken(jwtString)
client.RefreshAuth(ctx)
```

Tokens are cached in the system keychain (macOS) or encrypted config file at `~/.config/otf-cli/config.json`.

### Key types

| Type | File | Purpose |
|------|------|---------|
| `Client` | `otf_api.go` | Main API client |
| `CLIConfig` | `config.go` | Config model (preferred studios, tokens, timezone, consent prefs) |
| `Authenticator` interface | `auth.go` | Auth provider interface |
| `cognitoAuthenticator` | `cognito.go` | Cognito implementation |
| `Studio`, `ListStudiosResponse` | `studios.go` | Studio search types |
| `StudioClass`, `StudioScheduleResponse` | `schedule.go` | Schedule types |
| `BookingRequest`, `CreateBookingRequest` | `booking.go` | Booking types |
| `Middleware` | `middleware.go` | HTTP middleware for auth headers + auto-refresh on 401 |

### API Methods

| Method | File | Endpoint |
|--------|------|----------|
| `ListStudios(ctx, lat, long, distance)` | `studios.go` | `CO/mobile/v1/studios` |
| `GetStudiosSchedules(ctx, studioIDs)` | `schedule.go` | `IO/v1/classes` |
| `GetClassTypeFilter(ctx)` | `schedule.go` | `IO/v1/classes/filters` |
| `BookClass(ctx, CreateBookingRequest)` | `booking.go` | `IO/v1/bookings/me` POST |
| `GetBookings(ctx, start, end, includeCanceled)` | `booking.go` | `IO/v1/bookings/me` GET |
| `CancelBooking(ctx, bookingID)` | `booking.go` | `IO/v1/bookings/me/{id}` DELETE |

### Auth middleware
`AuthMiddleware` auto-refreshes the JWT on 401 responses if a refresh token is available.

## CLI (`cmd/otf-cli`)

Cobra-based CLI. Key commands:

- `auth` — interactive auth, saves to keychain
- `schedules` — browse/book classes (interactive or with `--class-id --book`)
- `bookings list` / `bookings cancel` — manage bookings
- `configure studios` — search and save preferred studio IDs
- `configure timezone` — set display timezone

Interactive mode uses `survey` library. Color-coded output uses `ansi`.

Config lives in `~/.config/otf-cli/config.json` or system keychain.

## MCP (`cmd/otf-mcp`)

JSON-RPC server over stdin/stdout (MCP protocol). Tools:

| Tool | Description |
|------|-------------|
| `get_schedules` | Fetch class schedules (optional `studio_ids`) |
| `list_bookings` | List upcoming bookings |
| `cancel_booking` | Cancel by `booking_id` |
| `book_class` | Book by `class_id` (optional `waitlist`) |
| `search_studios` | Search studios by `lat`/`long`/`distance` or IP location |

IP location detection requires user consent (`allow_ip_location` flag or config).

## Testing

Table-driven tests with `testify/require`. Test files co-located with source (`_test.go`). Middleware/auth tests use mocked HTTP servers.

## Code conventions

- No external comments in code (agents should not add comments)
- Follow existing patterns for new endpoints (typed response structs, `Client` method, JSON decoder)
- Group types by domain in dedicated files
- HTTP calls go through `Client.HTTPClient` to get auth middleware
