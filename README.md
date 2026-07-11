# OTF API

> A Go SDK, CLI, and MCP server for the OrangeTheory Fitness API — browse schedules, book classes, and manage your membership from your terminal.

[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://golang.org)
[![Go Reference](https://pkg.go.dev/badge/github.com/ammiranda/otf_api.svg)](https://pkg.go.dev/github.com/ammiranda/otf_api)
[![CI](https://github.com/ammiranda/otf_api/actions/workflows/ci.yaml/badge.svg)](https://github.com/ammiranda/otf_api/actions/workflows/ci.yaml)
[![Install CLI](https://img.shields.io/badge/go%20install-otf--cli-blue)](https://pkg.go.dev/github.com/ammiranda/otf_api/cmd/otf-cli)
[![Install MCP](https://img.shields.io/badge/go%20install-otf--mcp-blue)](https://pkg.go.dev/github.com/ammiranda/otf_api/cmd/otf-mcp)

> **⚠️ Unofficial project.** This is not affiliated with, endorsed by, or supported by OrangeTheory Fitness, LLC in any way. It reverse-engineers undocumented, private endpoints used by the official OTF app, which may violate OTF's Terms of Service. Endpoints can change or break without notice, and use is entirely at your own risk. Don't use this to abuse booking/waitlist systems or otherwise harm other members' access to classes.

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

## Prerequisites

You need an active OrangeTheory Fitness membership with online booking access (the same email/password you use for the OTF app or website).

**Authentication is required** before any API calls work. Choose one:

| Method | Best for |
|--------|----------|
| Run `otf-cli auth` (interactive prompt, saves to keychain) | CLI users & anyone who can run the CLI once before using MCP |
| Set `OTF_USERNAME` and `OTF_PASSWORD` env vars | MCP server users who can't/won't run the CLI first |

> **⚠️ MCP server users:** The MCP server **cannot prompt you for credentials** — it runs headless over stdin/stdout. You **must** either (a) run `otf-cli auth` first (the MCP server shares the same keychain), or (b) set `OTF_USERNAME` and `OTF_PASSWORD` in your MCP client config. If neither is done, every tool call will fail with "Authentication required: no credentials available".

## Common first-time gotchas

- **"Authentication required" from MCP:** You haven't run `otf-cli auth` or set env vars. See Prerequisites above.
- **"No studio IDs provided":** You haven't configured preferred studios. Run `otf-cli configure studios` or pass `studio_ids` explicitly to `get_schedules`.
- **macOS Gatekeeper blocks the binary:** Run `xattr -d com.apple.quarantine $(which otf-mcp)` (see Install section).
- **Keychain not available (SSH/Docker/CI):** Set `OTF_USERNAME` and `OTF_PASSWORD` env vars — the config file fallback needs these.
- **Token expired / session revoked:** If you get auth errors after a previous working session, tokens expire after 1 hour. Auto-refresh handles this normally, but if the refresh token is also stale, just re-run `otf-cli auth` or restart the MCP client (it will re-auth with env vars).

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

### 2. Authenticate

```bash
otf-cli auth
```

This prompts for your OTF email and password, authenticates with the API, and
stores your session securely in the system keychain. Future commands reuse the
cached session (tokens refresh automatically).

> **No keychain?** Set `OTF_USERNAME` and `OTF_PASSWORD` environment variables instead.
> `OTF_CLIENT_ID` is optional — it defaults to the iOS app client ID.

> **Using the MCP server?** You don't need to re-authenticate — `otf-mcp` shares the
> same keychain config. Just run `otf-cli auth` once and the MCP server picks it up.
> If you can't run the CLI at all, set `OTF_USERNAME`/`OTF_PASSWORD` in your MCP client config
> instead (see [MCP Server](#mcp-server) below).

### 3. Configure your studios

```bash
otf-cli configure studios
```

With your consent (asked once, saved to config), this auto-detects your location from your IP and lets you select which studios to track. You can also provide explicit lat/long coordinates with `--lat`, `--long`.

### 4. Browse & book

```bash
# View schedules
otf-cli schedules

# JSON output (pipe to jq)
otf-cli schedules --json | jq

# Book a specific class non-interactively
otf-cli schedules --class-id "<class-id>" --book --yes

# List your bookings
otf-cli bookings list

# Cancel a booking
otf-cli bookings cancel <booking-id> --yes
```

## CLI Reference

| Command | Description |
|---------|-------------|
| `auth` | Authenticate and save credentials to system keychain |
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

### Authentication (important)

The MCP server runs **headless** — it communicates over stdin/stdout JSON-RPC, not a terminal. This means:

- **It cannot prompt you for credentials interactively.**
- You **must** have credentials available from one of:
  1. **Run `otf-cli auth` first** — the MCP server reads the same keychain/config. This is the recommended approach.
  2. **Set `OTF_USERNAME` and `OTF_PASSWORD` env vars** in your MCP client config (see below).

If neither is configured, every tool call will fail with:
```
Authentication required: no credentials available
```

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
| `search_studios` | Search for studios near lat/lng or approximate IP-based location (optionally within a radius, defaults to 10 miles; requires consent for IP) |

### Claude Desktop Setup

**With keychain (recommended — run `otf-cli auth` first):**

```json
{
  "mcpServers": {
    "otf": {
      "command": "/path/to/bin/otf-mcp"
    }
  }
}
```

**With environment variables (no `otf-cli auth` needed):**

```json
{
  "mcpServers": {
    "otf": {
      "command": "/path/to/bin/otf-mcp",
      "env": {
        "OTF_USERNAME": "your@email.com",
        "OTF_PASSWORD": "your-password"
      }
    }
  }
}
```

All API URLs and the client ID have sensible defaults — no other configuration needed.

The server reads the same config as the CLI for preferred studios and cached tokens, so you only need to configure studios once.

### Cline / Cursor / Other MCP Clients

Same pattern — point the MCP client at the `otf-mcp` binary. If you've authenticated
with `otf-cli auth`, no env vars are needed.

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
    client := otf_api.NewClient()

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
| `OTF_USERNAME` | No | — | OTF account email (or use `otf-cli auth`) |
| `OTF_PASSWORD` | No | — | OTF account password (or use `otf-cli auth`) |
| `OTF_CLIENT_ID` | No | `65knvqta6p37efc2l3eh26pl5o` | Cognito app client ID (iOS app) |
| `OTF_API_IO_BASE_URL` | No | `https://api.orangetheory.io/v1/` | Classes & bookings API |
| `OTF_API_CO_BASE_URL` | No | `https://api.orangetheory.co/mobile/v1/` | Studios API |
| `OTF_AUTH_URL` | No | `https://cognito-idp.us-east-1.amazonaws.com/` | Cognito auth endpoint |

> Credentials and session tokens are cached in the system keychain automatically. Run `otf-cli auth` once and you're set.

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
- **macOS Keychain** — tokens are stored in the system keychain when available, falling back to an AES-256-GCM encrypted `~/.config/otf-cli/config.json` with a randomly generated, locally stored key. This fallback is meant for environments without keychain access (SSH, Docker, CI) — it's not a substitute for OS-level credential storage, since anyone with read access to your config directory can read the key file too.

## Project Structure

```
.
├── otf_api/            # Go SDK package
│   ├── otf_api.go      # Client constructor, env loading
│   ├── auth.go         # Authentication flows
│   ├── cognito.go      # Cognito authenticator
│   ├── config.go       # Config save/load (file + keychain)
│   ├── keyring.go      # macOS Keychain integration
│   ├── middleware.go   # HTTP middleware (auth headers, retry)
│   ├── studios.go      # Studio search
│   ├── schedule.go     # Class schedule fetching
│   ├── booking.go      # Booking CRUD
│   └── *_test.go       # Test files alongside each source
├── cmd/
│   ├── otf-cli/        # Interactive CLI (cobra + survey)
│   └── otf-mcp/        # MCP JSON-RPC server
├── bin/                # Build output (gitignored)
└── Makefile            # Build targets
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
