# OTF API

A Golang SDK and CLI for the OrangeTheory Fitness API that allows you to manage your class bookings, view schedules, and configure preferred studios.

## Features

- **Authentication**: Login with your OTF credentials
- **Studio Management**: Search and configure preferred studios by location
- **Class Scheduling**: View available classes at your preferred studios
- **Booking Management**: Book, view, and cancel class reservations
- **Interactive CLI**: User-friendly command-line interface with selections and confirmations

## Installation

```bash
# Build the CLI
make build-cli

# The binary will be available at ./bin/otf-cli
```

## Configuration

### Environment Variables

Create a `.env` file in the project root:

```bash
OTF_USERNAME=your_email@example.com
OTF_PASSWORD=your_password
OTF_CLIENT_ID=your_client_id
```

### Configure Preferred Studios

```bash
# Search for studios near your location and save preferences
./bin/otf-cli configure studios

# Set your preferred timezone for class times
./bin/otf-cli configure timezone
```

The `configure studios` command will:
1. Auto-detect your location (or prompt for manual input)
2. Search for OTF studios within your specified distance
3. Let you select multiple preferred studios
4. Save these preferences for future use

## Usage

### View Class Schedules

```bash
# View schedules for your configured preferred studios
./bin/otf-cli schedules

# Or specify specific studio IDs
./bin/otf-cli schedules --studio-ids "studio-id-1,studio-id-2"
```

This command will:
- Show available classes with color-coded studios
- Display class times in your configured timezone
- Allow you to select and book a class interactively
- Handle waitlist booking for full classes

### Manage Bookings

```bash
# View and manage your current bookings
./bin/otf-cli bookings list
```

The bookings list command provides an interactive menu where you can:
- **Select a booking to cancel** - Choose from active bookings and confirm cancellation
- **Just view bookings** - See all your bookings (active and canceled) without taking action
- **Filter to future bookings only** - Only shows today and upcoming classes

### Cancel Specific Booking (Alternative)

```bash
# Cancel a booking by ID (if you know the booking ID)
./bin/otf-cli bookings cancel <booking-id>
```

## Command Reference

### Core Commands

- `configure studios` - Search and configure preferred OTF studios
- `configure timezone` - Set your preferred timezone for class display
- `schedules` - View and book classes at your studios
- `bookings list` - Interactive booking management (view/cancel)
- `bookings cancel <id>` - Cancel a specific booking by ID

### Configuration Files

The CLI stores configuration in your system's config directory:
- **Studios**: Preferred studio IDs for quick access
- **Timezone**: Display classes in your preferred timezone

## API Endpoints

The SDK interacts with these OrangeTheory Fitness API endpoints:

- `POST /v1/auth` - Authentication
- `GET /v1/studios` - Studio search by location
- `GET /v1/schedules` - Class schedules for studios  
- `POST /v1/bookings/me` - Book a class
- `GET /v1/bookings/me` - List user bookings
- `DELETE /v1/bookings/me/{id}` - Cancel a booking

## Example Workflow

1. **Initial Setup**:
   ```bash
   # Set up environment variables
   echo "OTF_USERNAME=your_email@example.com" > .env
   echo "OTF_PASSWORD=your_password" >> .env
   echo "OTF_CLIENT_ID=your_client_id" >> .env
   
   # Configure preferred studios
   ./bin/otf-cli configure studios
   ```

2. **Daily Usage**:
   ```bash
   # Check and book classes
   ./bin/otf-cli schedules
   
   # Manage existing bookings
   ./bin/otf-cli bookings list
   ```

## Development

### Building

```bash
# Build CLI
make build-cli

# Build library
make build-lib

# Run tests
make test
```

### Project Structure

- `otf_api/` - Core Go SDK library
- `cmd/otf-cli/` - CLI application
- `Makefile` - Build configuration

## Notes

- The API uses JWT authentication tokens that expire
- Cross-regional booking is supported (book at studios outside your home location)
- Class booking has time restrictions (usually 7-30 days in advance depending on studio/membership)
- Some classes may require specific membership types or have waitlists