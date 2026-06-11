# OTF API

> A Go SDK, CLI, and MCP server for the OrangeTheory Fitness API — browse schedules, book classes, and manage your membership from your terminal.

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://golang.org)
[![Go Reference](https://pkg.go.dev/badge/github.com/ammiranda/otf_api.svg)](https://pkg.go.dev/github.com/ammiranda/otf_api)
[![CI](https://github.com/ammiranda/otf_api/actions/workflows/ci.yaml/badge.svg)](https://github.com/ammiranda/otf_api/actions/workflows/ci.yaml)
[![Install CLI](https://img.shields.io/badge/go%20install-otf--cli-blue)](https://pkg.go.dev/github.com/ammiranda/otf_api/cmd/otf-cli)
[![Install MCP](https://img.shields.io/badge/go%20install-otf--mcp-blue)](https://pkg.go.dev/github.com/ammiranda/otf_api/cmd/otf-mcp)

## What is this?

OrangeTheory Fitness doesn't provide an official API, but their app communicates with internal APIs. This project reverse-engineers those endpoints to give you programmatic access to:

- **Browse class schedules** across multiple studios
- **Book classes** (including waitlists)
- **Cancel bookings**
- **Search for studios** by location

It comes in three flavors:

| Component | What it does |
|-----------|-------------|
| **`otf-api`** (Go SDK) | Core library — embed OTF access in your own Go programs |
| **`otf-cli`** (CLI) | Interactive terminal app — browse & book with colored output |
| **`otf-mcp`** (MCP Server) | AI-ready JSON-RPC server — use with Claude Desktop, Cline, etc. |

## Demo

```
$ otf-cli schedules

  === Tue Jun 2 ===
  Orange 60 Min 2G    5:00 AM CDT   6:00 AM CDT   Mueller, TX
  Orange 60 Min 2G    5:00 AM CDT   6:00 AM CDT   Austin - Four Points, TX
  Orange 60 Min 2G    6:10 AM CDT   7:10 AM CDT   Triangle, TX
  Orange 60 Min 3G    6:15 AM CDT   7:15 AM CDT   Mueller, TX
  Orange 60 Min 2G    6:15 AM CDT   7:15 AM CDT   South Lamar, TX
  ...

  ? Select a class to book: [Use arrows to move, type to filter]
```

## Quick Start

### 1. Install

**Go (requires Go 1.26+):**
```bash
go install github.com/ammiranda/otf_api/cmd/otf-cli@latest
go install github.com/ammiranda/otf_api/cmd/otf-mcp@latest
```

**Homebrew (tap):** (requires [`ammiranda/homebrew-tap`](https://github.com/ammiranda/homebrew-tap))
```bash
brew install ammiranda/tap/otf-cli
brew install ammiranda/tap/otf-mcp
```

> **macOS Gatekeeper:** binaries are not signed with an Apple Developer ID, so macOS may flag them as unidentified. Run `xattr -d com.apple.quarantine $(which otf-mcp)` (or `otf-cli`) to remove the quarantine attribute. Alternatively, right-click the binary in Finder and select **Open** the first time.

**Build from source:**
```bash
git clone https://github.com/ammiranda/otf_api.git
cd otf_api
make build-cli    # produces bin/otf-cli
make build-mcp    # produces bin/otf-mcp
```

### 2. Configure credentials

Copy `.env.example` to `.env` and fill in your details:

```bash
cp .env.example .env
```

You need two things:
- `OTF_USERNAME` — your OTF account email
- `OTF_PASSWORD` — your OTF account password

> `OTF_CLIENT_ID` is optional — it defaults to the iOS app client ID. Only set it if the default stops working.

### 3. Configure your studios

```bash
./bin/otf-cli configure studios
```

This auto-detects your location from your IP and lets you select which studios to track.

### 4. Browse & book

```bash
# View schedules
./bin/otf-cli schedules

# JSON output (pipe to jq)
./bin/otf-cli schedules --json | jq

# Book a specific class non-interactively
./bin/otf-cli schedules --class-id "<class-id>" --book --yes

# List your bookings
./bin/otf-cli bookings list

# Cancel a booking
./bin/otf-cli bookings cancel <booking-id> --yes
```

## CLI Reference

| Command | Description |
|---------|-------------|
| `configure studios` | Search & save preferred OTF studios |
| `configure timezone` | Set your display timezone |
| `schedules` | View & interactively book classes |
| `schedules --json` | Schedule as JSON |
| `schedules --class-id <id> --book --yes` | Book a class non-interactively |
| `bookings list` | List bookings (interactive cancel) |
| `bookings list --json` | Bookings as JSON |
| `bookings cancel <id> --yes` | Cancel by booking ID |

### Flags

- `--studio-ids "id1,id2"` — fetch schedules for specific studios
- `--json` — output as JSON (stdout); logs go to stderr
- `--yes` — skip confirmation prompts (for scripting)

## MCP Server

The `otf-mcp` server exposes OTF functionality as MCP tools, letting AI assistants (Claude Desktop, Cline, etc.) look up classes and manage bookings on your behalf.

### Build

```bash
make build-mcp
# produces bin/otf-mcp
```

### MCP Tools

| Tool | Description |
|------|-------------|
| `get_schedules` | Fetch class schedules by studio IDs (or use preferred studios from config) |
| `list_bookings` | List your current/upcoming bookings |
| `book_class` | Book a class by class ID (optional waitlist) |
| `cancel_booking` | Cancel a booking by ID |
| `search_studios` | Search for studios near lat/lng coordinates |

### Claude Desktop Setup

Add to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "otf": {
      "command": "/path/to/bin/otf-mcp",
      "env": {
        "OTF_USERNAME": "your@email.com",
        "OTF_PASSWORD": "your-password",
        "OTF_API_IO_BASE_URL": "https://api.orangetheory.io/v1/",
        "OTF_API_CO_BASE_URL": "https://api.orangetheory.co/mobile/v1/",
        "OTF_AUTH_URL": "https://cognito-idp.us-east-1.amazonaws.com/"
      }
    }
  }
}
```

The server reads the same config file as the CLI for preferred studios and cached tokens, so you only need to configure studios once.

### Cline / Cursor / Other MCP Clients

Same pattern — point the MCP client at the `otf-mcp` binary with the environment variables above.

## SDK Usage (Go)

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/ammiranda/otf_api/otf_api"
)

func main() {
    client, err := otf_api.NewClient()
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()
    if err := client.Authenticate(ctx, "email", "password"); err != nil {
        log.Fatal(err)
    }

    // List studios near Austin, TX
    studios, _ := client.ListStudios(ctx, 30.27, -97.74, 10)
    for _, s := range studios.Data.Data {
        fmt.Printf("%s (%.2f mi)\n", s.StudioName, s.Distance)
    }
}
```

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OTF_USERNAME` | No* | — | Your OTF account email |
| `OTF_PASSWORD` | No* | — | Your OTF account password |
| `OTF_CLIENT_ID` | No | `65knvqta6p37efc2l3eh26pl5o` | Cognito app client ID (iOS app) |
| `OTF_API_IO_BASE_URL` | No | `https://api.orangetheory.io/v1/` | Classes & bookings API |
| `OTF_API_CO_BASE_URL` | No | `https://api.orangetheory.co/mobile/v1/` | Studios API |
| `OTF_AUTH_URL` | No | `https://cognito-idp.us-east-1.amazonaws.com/` | Cognito auth endpoint |

\* Credentials only needed on first run; tokens are cached to `~/.config/otf-cli/config.json`
  (macOS Keychain is used automatically when available).

## How It Works

```
┌─────────────┐     ┌──────────────┐     ┌──────────────────────┐
│  otf-cli    │────▶│              │     │                      │
│  (terminal) │     │  otf_api     │────▶│  OrangeTheory API    │
├─────────────┤     │  (Go SDK)    │     │  (AWS Cognito Auth)  │
│  otf-mcp    │────▶│              │     │                      │
│  (MCP/s    ) │     └──────────────┘     └──────────────────────┘
└─────────────┘
```

The SDK handles:
- **Authentication** via AWS Cognito (username/password + refresh tokens)
- **Token caching** — avoids re-login on every command
- **Automatic token refresh** on 401 responses
- **Gzip decoding** for API responses
- **macOS Keychain** — tokens are stored in the system keychain when available, falling back to `~/.config/otf-cli/config.json`

## Project Structure

```
.
├── otf_api.go          # Client constructor, env loading
├── auth.go             # Authentication flows
├── cognito.go          # Cognito authenticator
├── middleware.go       # HTTP middleware (auth headers, retry)
├── studios.go          # Studio search
├── schedule.go         # Class schedule fetching
├── booking.go          # Booking CRUD
├── cmd/
│   ├── otf-cli/        # Interactive CLI (cobra + survey)
│   └── otf-mcp/        # MCP JSON-RPC server
├── Makefile            # Build targets
└── .env.example        # Environment template
```

## Development

```bash
make build       # Build everything
make build-cli   # Build just the CLI (bin/otf-cli)
make build-mcp   # Build just the MCP server (bin/otf-mcp)
make test        # Run tests
make lint        # Run golangci-lint
```

## License

MIT
