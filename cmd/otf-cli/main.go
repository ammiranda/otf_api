package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/ammiranda/otf_api/otf_api"
	"github.com/joho/godotenv"
	"github.com/mgutz/ansi"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	configFileName = "config.json"
	cliDirName     = "otf-cli"
)

// IPLocation represents the response from ip-api.com
type IPLocation struct {
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	City    string  `json:"city"`
	Region  string  `json:"regionName"`
	Country string  `json:"country"`
}

// CLIConfig holds the CLI configuration
type CLIConfig struct {
	PreferredStudioIDs []string `json:"preferred_studio_ids,omitempty"`
	Timezone           string   `json:"timezone,omitempty"`
	Token              string   `json:"token,omitempty"`
	RefreshToken       string   `json:"refresh_token,omitempty"`
}

var rootCmd = &cobra.Command{
	Use:   "otf-cli",
	Short: "A CLI client for the OrangeTheory Fitness API",
	Long:  `otf-cli is a command-line interface to interact with the OrangeTheory Fitness API, allowing users to fetch schedules and other information.`,
}

var studioIDs string

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure otf-cli settings",
	Long:  `Provides commands to configure various settings for the otf-cli, such as preferred studios.`,
}

var (
	configureStudioJSON bool
	configureStudioLat  float64
	configureStudioLong float64
	configureStudioDist float64
	configureStudioSave bool
)

var configureStudiosCmd = &cobra.Command{
	Use:   "studios",
	Short: "Configure preferred OTF studios",
	Long: `Allows you to search for OTF studios by location and save your preferred ones.

With --lat, --long, --distance, runs non-interactively.
With --json, outputs the studio list as JSON.
With --save, saves the selected studio IDs as preferred.

Examples:
  otf-cli configure studios --lat 40.7128 --long -74.0060 --distance 10 --json
  otf-cli configure studios --lat 40.7128 --long -74.0060 --distance 10 --json --save`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		apiClient := setupClient(ctx)

		var lat, long, dist float64
		useFlags := cmd.Flags().Changed("lat") || cmd.Flags().Changed("long") || cmd.Flags().Changed("distance")

		if useFlags {
			lat = configureStudioLat
			long = configureStudioLong
			dist = configureStudioDist
			if lat == 0 && long == 0 {
				log.Fatal("--lat and --long must be provided")
			}
			if dist <= 0 {
				dist = 10
			}
		} else {
			// Interactive location detection
			var locationSource string

			resp, err := http.Get("http://ip-api.com/json/")
			if err == nil {
				defer func() {
					if err := resp.Body.Close(); err != nil {
						log.Printf("error closing response body: %v", err)
					}
				}()
				var location IPLocation
				if err := json.NewDecoder(resp.Body).Decode(&location); err == nil {
					lat = location.Lat
					long = location.Lon
					locationSource = fmt.Sprintf("detected from your IP in %s, %s, %s",
						location.City, location.Region, location.Country)
				}
			}
			if err != nil || locationSource == "" {
				log.Printf("Warning: Could not detect location from IP: %v", err)
			}

			if lat == 0 && long == 0 {
				locationQs := []*survey.Question{
					{Name: "latitude", Prompt: &survey.Input{Message: "Enter your latitude (e.g., 40.7128):"}, Validate: survey.Required},
					{Name: "longitude", Prompt: &survey.Input{Message: "Enter your longitude (e.g., -74.0060):"}, Validate: survey.Required},
				}
				locationAnswers := struct {
					Latitude  string `survey:"latitude"`
					Longitude string `survey:"longitude"`
				}{}
				if err := survey.Ask(locationQs, &locationAnswers); err != nil {
					log.Fatalf("Error getting location input: %v", err)
				}
				var errLat, errLong error
				lat, errLat = strconv.ParseFloat(locationAnswers.Latitude, 64)
				long, errLong = strconv.ParseFloat(locationAnswers.Longitude, 64)
				if errLat != nil || errLong != nil {
					log.Fatalf("Invalid numeric input for latitude or longitude.")
				}
				locationSource = "manually entered"
			}

			distanceQs := []*survey.Question{
				{Name: "distance", Prompt: &survey.Input{Message: "Enter search distance in miles (e.g., 10):"}, Validate: survey.Required},
			}
			distanceAnswers := struct {
				Distance string `survey:"distance"`
			}{}
			if err := survey.Ask(distanceQs, &distanceAnswers); err != nil {
				log.Fatalf("Error getting distance input: %v", err)
			}
			dist, err = strconv.ParseFloat(distanceAnswers.Distance, 64)
			if err != nil {
				log.Fatalf("Invalid numeric input for distance.")
			}
			log.Printf("Using location %s: %.6f, %.6f", locationSource, lat, long)
		}

		log.Printf("Fetching studios near %.6f, %.6f (%.1f miles)...", lat, long, dist)
		studioListResponse, err := apiClient.ListStudios(ctx, lat, long, dist)
		if err != nil {
			log.Fatalf("Error fetching studios: %v", err)
		}

		if configureStudioJSON {
			writeJSON(studioListResponse)
		}

		if configureStudioSave && len(studioListResponse.Data.Data) > 0 {
			ids := make([]string, len(studioListResponse.Data.Data))
			for i, s := range studioListResponse.Data.Data {
				ids[i] = s.StudioUUID
			}
			config, err := loadConfig()
			if err != nil {
				log.Printf("Warning: Could not load config: %v", err)
				config = CLIConfig{}
			}
			config.PreferredStudioIDs = ids
			if err := saveConfig(config); err != nil {
				log.Fatalf("Error saving configuration: %v", err)
			}
			log.Printf("Saved %d studios as preferred", len(ids))
		}

		if !configureStudioJSON && !configureStudioSave {
			// Interactive mode
			if len(studioListResponse.Data.Data) == 0 {
				log.Println("No studios found.")
				return
			}

			studioOptions := []string{}
			studioMap := make(map[string]string)
			for _, studio := range studioListResponse.Data.Data {
				displayName := fmt.Sprintf("%s (ID: %s, %.2f miles)", studio.StudioName, studio.StudioUUID, studio.Distance)
				studioOptions = append(studioOptions, displayName)
				studioMap[displayName] = studio.StudioUUID
			}

			selectedDisplayNames := []string{}
			prompt := &survey.MultiSelect{
				Message:  "Select your preferred studios:",
				Options:  studioOptions,
				PageSize: 15,
			}
			if err := survey.AskOne(prompt, &selectedDisplayNames); err != nil {
				log.Fatalf("Error during studio selection: %v", err)
			}

			selectedStudioIDs := []string{}
			for _, displayName := range selectedDisplayNames {
				if id, ok := studioMap[displayName]; ok {
					selectedStudioIDs = append(selectedStudioIDs, id)
				}
			}

			config, err := loadConfig()
			if err != nil {
				config = CLIConfig{}
			}
			config.PreferredStudioIDs = selectedStudioIDs
			if err := saveConfig(config); err != nil {
				log.Fatalf("Error saving configuration: %v", err)
			}

			if len(selectedStudioIDs) > 0 {
				log.Printf("Preferred studios saved: %s", strings.Join(selectedStudioIDs, ", "))
			} else {
				log.Println("No studios selected.")
			}
		}
	},
}

var configureTimezoneFlag string

var configureTimezoneCmd = &cobra.Command{
	Use:   "timezone",
	Short: "Configure your preferred timezone",
	Long: `Set your preferred timezone for displaying class times.

With --timezone, sets it directly without an interactive prompt.
Use --timezone "" to clear and use system local timezone.

Examples:
  otf-cli configure timezone --timezone "America/New_York"
  otf-cli configure timezone --timezone ""`,
	Run: func(cmd *cobra.Command, args []string) {
		config, err := loadConfig()
		if err != nil {
			log.Printf("Warning: Could not load existing config, will create a new one: %v", err)
			config = CLIConfig{}
		}

		if cmd.Flags().Changed("timezone") {
			config.Timezone = configureTimezoneFlag
			if err := saveConfig(config); err != nil {
				log.Fatalf("Error saving configuration: %v", err)
			}
			if config.Timezone == "" {
				fmt.Println("Timezone set to use system local timezone.")
			} else {
				fmt.Printf("Timezone set to: %s\n", config.Timezone)
			}
			return
		}

		// Interactive mode
		timezones := []string{
			"America/New_York",
			"America/Chicago",
			"America/Denver",
			"America/Los_Angeles",
			"America/Anchorage",
			"Pacific/Honolulu",
			"America/Phoenix",
			"America/Detroit",
			"America/Indiana/Indianapolis",
			"America/Kentucky/Louisville",
			"America/Boise",
			"America/Seattle",
			"America/Portland",
		}

		if config.Timezone != "" {
			found := false
			for _, tz := range timezones {
				if tz == config.Timezone {
					found = true
					break
				}
			}
			if !found {
				timezones = append(timezones, config.Timezone)
			}
		}

		timezones = append(timezones, "System Local Timezone")

		var selectedTimezone string
		prompt := &survey.Select{
			Message: "Select your preferred timezone:",
			Options: timezones,
			Default: func() string {
				if config.Timezone == "" {
					return "System Local Timezone"
				}
				for _, tz := range timezones {
					if tz == config.Timezone {
						return tz
					}
				}
				return "System Local Timezone"
			}(),
		}
		if err := survey.AskOne(prompt, &selectedTimezone); err != nil {
			log.Fatalf("Error during timezone selection: %v", err)
		}

		if selectedTimezone == "System Local Timezone" {
			config.Timezone = ""
		} else {
			config.Timezone = selectedTimezone
		}

		if err := saveConfig(config); err != nil {
			log.Fatalf("Error saving configuration: %v", err)
		}

		if config.Timezone == "" {
			fmt.Println("Timezone set to use system local timezone.")
		} else {
			fmt.Printf("Timezone set to: %s\n", config.Timezone)
		}
	},
}

var (
	listBookingsJSON       bool
	listBookingsCancelID   string
	listBookingsCancelFlag bool
	listBookingsYes        bool
	cancelBookingYes       bool
)

var bookingsCmd = &cobra.Command{
	Use:   "bookings",
	Short: "Manage your OTF bookings",
	Long:  `Commands to list and cancel your OrangeTheory Fitness bookings.`,
}

var listBookingsCmd = &cobra.Command{
	Use:   "list",
	Short: "List your current bookings",
	Long: `Lists all your current and upcoming OrangeTheory Fitness bookings.

With --json, outputs bookings as JSON to stdout.
With --cancel and --booking-id and --yes, cancels non-interactively.

Examples:
  otf-cli bookings list --json
  otf-cli bookings list --cancel --booking-id "abc" --yes --json`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		apiClient := setupClient(ctx)

		startsAfter := time.Now().Truncate(24 * time.Hour)
		endsBefore := time.Now().AddDate(0, 0, 60)

		bookings, err := apiClient.GetBookings(ctx, startsAfter, endsBefore, true)
		if err != nil {
			log.Fatalf("Error fetching bookings: %v", err)
		}

		// Non-interactive cancel via --cancel --booking-id
		if listBookingsCancelFlag && listBookingsCancelID != "" {
			if !listBookingsYes {
				var shouldCancel bool
				prompt := &survey.Confirm{
					Message: fmt.Sprintf("Are you sure you want to cancel booking %s?", listBookingsCancelID),
				}
				if err := survey.AskOne(prompt, &shouldCancel); err != nil {
					log.Fatalf("Error during cancellation confirmation: %v", err)
				}
				if !shouldCancel {
					fmt.Println("Cancellation aborted.")
					return
				}
			}

			err = apiClient.CancelBooking(ctx, listBookingsCancelID)
			if err != nil {
				log.Fatalf("Error canceling booking: %v", err)
			}

			if listBookingsJSON {
				writeJSON(map[string]string{"status": "canceled", "booking_id": listBookingsCancelID})
				return
			}
			fmt.Printf("Successfully canceled booking %s\n", listBookingsCancelID)
			return
		}

		// JSON output
		if listBookingsJSON {
			writeJSON(bookings)
			return
		}

		// Interactive mode (existing behavior)
		if len(bookings) == 0 {
			fmt.Println("No bookings found.")
			return
		}

		config, err := loadConfig()
		if err != nil {
			log.Printf("Warning: Could not load configuration: %v", err)
			config = CLIConfig{}
		}

		activeBookings := []otf_api.BookingRequest{}
		for _, booking := range bookings {
			if !booking.Canceled {
				activeBookings = append(activeBookings, booking)
			}
		}

		if len(activeBookings) == 0 {
			fmt.Println("No active bookings found.")
			return
		}

		bookingOptions := []string{}
		bookingMap := make(map[string]otf_api.BookingRequest)

		for _, booking := range activeBookings {
			classTime, err := time.Parse(time.RFC3339, booking.Class.StartsAt)
			if err != nil {
				classTime = time.Now()
			}

			displayTime := classTime
			if config.Timezone != "" {
				if loc, err := time.LoadLocation(config.Timezone); err == nil {
					displayTime = classTime.In(loc)
				}
			}
			dayStr := displayTime.Format("Mon Jan 2")

			displayStr := fmt.Sprintf("%s - %s at %s - %s",
				dayStr,
				booking.Class.Name,
				booking.Class.Studio.Name,
				formatTime(classTime, config))

			bookingOptions = append(bookingOptions, displayStr)
			bookingMap[displayStr] = booking
		}

		bookingOptions = append(bookingOptions, "Just view bookings (no action)")

		var selectedBookingDisplay string
		prompt := &survey.Select{
			Message:  "Select a booking to cancel (or just view):",
			Options:  bookingOptions,
			PageSize: 15,
		}
		if err := survey.AskOne(prompt, &selectedBookingDisplay); err != nil {
			log.Fatalf("Error during booking selection: %v", err)
		}

		if selectedBookingDisplay == "Just view bookings (no action)" {
			fmt.Printf("\nYour Bookings (%d total):\n\n", len(bookings))
			lastDay := ""
			for i, booking := range bookings {
				status := "Booked"
				if booking.Canceled {
					status = ansi.Color("Canceled", "red")
				} else if booking.LateCanceled {
					status = ansi.Color("Late Canceled", "yellow")
				} else {
					status = ansi.Color("Booked", "green")
				}

				classTime, err := time.Parse(time.RFC3339, booking.Class.StartsAt)
				if err != nil {
					classTime = time.Now()
				}

				bookingDay := classTime.Format("Mon Jan 2")
				if config.Timezone != "" {
					if loc, err := time.LoadLocation(config.Timezone); err == nil {
						bookingDay = classTime.In(loc).Format("Mon Jan 2")
					}
				}

				if bookingDay != lastDay {
					if i > 0 {
						fmt.Println()
					}
					header := fmt.Sprintf("=== %s ===", bookingDay)
					fmt.Println(header)
					lastDay = bookingDay
				}

				fmt.Printf("%s\n", ansi.Color(booking.Class.Name, "cyan"))
				fmt.Printf("   Studio: %s\n", booking.Class.Studio.Name)
				fmt.Printf("   Time: %s\n", formatTime(classTime, config))
				fmt.Printf("   Status: %s\n", status)
				fmt.Printf("   Booking ID: %s\n", booking.ID)
				fmt.Println()
			}
			return
		}

		selectedBooking, ok := bookingMap[selectedBookingDisplay]
		if !ok {
			log.Fatal("Error: Selected booking not found")
		}

		classTime, _ := time.Parse(time.RFC3339, selectedBooking.Class.StartsAt)
		fmt.Printf("\nSelected Booking:\n")
		fmt.Printf("Class: %s\n", selectedBooking.Class.Name)
		fmt.Printf("Studio: %s\n", selectedBooking.Class.Studio.Name)
		fmt.Printf("Time: %s\n", formatTime(classTime, config))
		fmt.Printf("Booking ID: %s\n", selectedBooking.ID)

		var shouldCancel bool
		cancelPrompt := &survey.Confirm{
			Message: "Are you sure you want to cancel this booking?",
		}
		if err := survey.AskOne(cancelPrompt, &shouldCancel); err != nil {
			log.Fatalf("Error during cancellation confirmation: %v", err)
		}

		if !shouldCancel {
			fmt.Println("Cancellation aborted.")
			return
		}

		err = apiClient.CancelBooking(ctx, selectedBooking.ID)
		if err != nil {
			log.Fatalf("Error canceling booking: %v", err)
		}

		fmt.Printf("Successfully canceled booking for %s at %s\n",
			selectedBooking.Class.Name,
			selectedBooking.Class.Studio.Name)
	},
}

var cancelBookingCmd = &cobra.Command{
	Use:   "cancel [booking-id]",
	Short: "Cancel a booking",
	Long: `Cancel a booking by providing the booking ID.

With --yes, skips the confirmation prompt.

Examples:
  otf-cli bookings cancel <id> --yes`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		bookingID := args[0]
		ctx := context.Background()
		apiClient := setupClient(ctx)

		if !cancelBookingYes {
			var shouldCancel bool
			prompt := &survey.Confirm{
				Message: fmt.Sprintf("Are you sure you want to cancel booking %s?", bookingID),
			}
			if err := survey.AskOne(prompt, &shouldCancel); err != nil {
				log.Fatalf("Error during cancellation confirmation: %v", err)
			}
			if !shouldCancel {
				fmt.Println("Cancellation aborted.")
				return
			}
		}

		err := apiClient.CancelBooking(ctx, bookingID)
		if err != nil {
			log.Fatalf("Error canceling booking: %v", err)
		}

		fmt.Printf("Successfully canceled booking %s\n", bookingID)
	},
}

var (
	scheduleJSON     bool
	scheduleClassID  string
	scheduleBook     bool
	scheduleYes      bool
)

var schedulesCmd = &cobra.Command{
	Use:   "schedules",
	Short: "Fetch studio schedules",
	Long: `Fetches schedules for the specified studio IDs.

With --json, outputs the schedule as JSON to stdout (logs go to stderr).
With --class-id, --book, and --yes, books a class non-interactively.

Examples:
  otf-cli schedules --studio-ids "abc,def" --json
  otf-cli schedules --studio-ids "abc,def" --class-id "xyz" --book --yes --json`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		apiClient := setupClient(ctx)

		var idsToFetch []string

		if studioIDs != "" {
			idsToFetch = strings.Split(studioIDs, ",")
		} else {
			config, err := loadConfig()
			if err != nil {
				log.Fatalf("Error loading configuration: %v. Provide --studio-ids or run 'otf-cli configure studios'.", err)
			}
			if len(config.PreferredStudioIDs) > 0 {
				idsToFetch = config.PreferredStudioIDs
				log.Printf("Using preferred studio IDs from configuration: %s", strings.Join(idsToFetch, ", "))
			} else {
				log.Fatal("No studio IDs provided via --studio-ids and no preferred studios configured.")
			}
		}

		schedules, err := apiClient.GetStudiosSchedules(ctx, idsToFetch)
		if err != nil {
			log.Fatalf("Error fetching schedules: %v", err)
		}

		if scheduleJSON {
			writeJSON(schedules)
			return
		}

		// Interactive mode (existing behavior)
		if len(schedules.Items) == 0 {
			log.Println("No classes found for the selected studios.")
			return
		}

		config, err := loadConfig()
		if err != nil {
			log.Printf("Warning: Could not load configuration: %v", err)
			config = CLIConfig{}
		}

		classOptions := []string{}
		classMap := make(map[string]otf_api.StudioClass)

		studioColors := []string{"red", "green", "yellow", "blue", "magenta", "cyan", "white"}
		studioColorMap := make(map[string]string)
		colorIdx := 0

		termWidth := 80
		if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
			termWidth = w
		}
		var entryWidth int
		if termWidth >= 120 {
			entryWidth = 110
		} else if termWidth >= 100 {
			entryWidth = 90
		} else {
			entryWidth = 70
		}

		maxClassName := 0
		maxStartTime := 0
		maxEndTime := 0
		for _, class := range schedules.Items {
			if class.Canceled {
				continue
			}
			if l := len([]rune(class.Name)); l > maxClassName {
				maxClassName = l
			}
			startTime := formatTime(class.StartsAt, config)
			endTime := formatTime(class.EndsAt, config)
			if l := len([]rune(startTime)); l > maxStartTime {
				maxStartTime = l
			}
			if l := len([]rune(endTime)); l > maxEndTime {
				maxEndTime = l
			}
		}

		minSpace := 2
		studioColStart := maxClassName + minSpace + maxStartTime + minSpace + maxEndTime + minSpace
		studioColWidth := entryWidth - studioColStart
		if studioColWidth < 10 {
			studioColWidth = 10
		}

		lastDay := ""
		for _, class := range schedules.Items {
			if class.Canceled {
				continue
			}

			studioID := class.Studio.ID
			studioName := class.Studio.Name
			colorName, ok := studioColorMap[studioID]
			if !ok {
				colorName = studioColors[colorIdx%len(studioColors)]
				studioColorMap[studioID] = colorName
				colorIdx++
			}

			startTime := formatTime(class.StartsAt, config)
			endTime := formatTime(class.EndsAt, config)

			classDay := class.StartsAt.Format("Mon Jan 2")
			if config.Timezone != "" {
				if loc, err := time.LoadLocation(config.Timezone); err == nil {
					classDay = class.StartsAt.In(loc).Format("Mon Jan 2")
				}
			}

			if classDay != lastDay {
				header := fmt.Sprintf("=== %s ===", classDay)
				classOptions = append(classOptions, header)
				lastDay = classDay
			}

			coloredStudio := ansi.Color(studioName, colorName)

			classNameCol := padOrTruncate(class.Name, maxClassName)
			startTimeCol := padOrTruncate(startTime, maxStartTime)
			endTimeCol := padOrTruncate(endTime, maxEndTime)
			studioCol := padOrTruncate(coloredStudio, studioColWidth)

			displayStr := fmt.Sprintf("%s%s%s%s%s%s%s",
				classNameCol, strings.Repeat(" ", minSpace),
				startTimeCol, strings.Repeat(" ", minSpace),
				endTimeCol, strings.Repeat(" ", minSpace),
				studioCol,
			)

			displayStr = padOrTruncate(displayStr, entryWidth)

			classOptions = append(classOptions, displayStr)
			classMap[displayStr] = class
		}

		if len(classOptions) == 0 {
			log.Println("No available classes found for the selected studios.")
			return
		}

		// Non-interactive booking mode
		if scheduleClassID != "" && scheduleBook {
			selectedClass, ok := findClassByID(schedules.Items, scheduleClassID)
			if !ok {
				log.Fatalf("Class %s not found in schedule", scheduleClassID)
			}

			needsWaitlist := selectedClass.BookingCapacity <= 0

			if needsWaitlist && !scheduleYes {
				var useWaitlist bool
				waitlistPrompt := &survey.Confirm{
					Message: "This class is full. Would you like to join the waitlist?",
				}
				if err := survey.AskOne(waitlistPrompt, &useWaitlist); err != nil {
					log.Fatalf("Error during waitlist confirmation: %v", err)
				}
				if !useWaitlist {
					fmt.Println("Booking cancelled.")
					return
				}
			}

			bookingReq := otf_api.CreateBookingRequest{
				ClassID:   selectedClass.ID,
				Confirmed: false,
				Waitlist:  needsWaitlist,
			}

			err := apiClient.BookClass(ctx, bookingReq)
			if err != nil {
				log.Fatalf("Error booking class: %v", err)
			}

			if scheduleJSON {
				result := map[string]any{
					"status":   "booked",
					"class_id": selectedClass.ID,
					"name":     selectedClass.Name,
					"studio":   selectedClass.Studio.Name,
					"waitlist": needsWaitlist,
				}
				writeJSON(result)
				return
			}

			if needsWaitlist {
				fmt.Println("Successfully added to waitlist!")
			} else {
				fmt.Println("Successfully booked the class!")
			}
			return
		}

		// Interactive selection (existing behavior)
		var selectedClassDisplay string
		prompt := &survey.Select{
			Message:  "Select a class to book:",
			Options:  classOptions,
			PageSize: 15,
		}
		if err := survey.AskOne(prompt, &selectedClassDisplay); err != nil {
			log.Fatalf("Error during class selection: %v", err)
		}

		selectedClass, ok := classMap[selectedClassDisplay]
		if !ok {
			log.Fatal("Error: Selected class not found in class map")
		}

		fmt.Printf("\nSelected Class Details:\n")
		fmt.Printf("Class: %s\n", selectedClass.Name)
		fmt.Printf("Studio: %s\n", selectedClass.Studio.Name)
		fmt.Printf("Time: %s to %s\n",
			formatTime(selectedClass.StartsAt, config),
			formatTime(selectedClass.EndsAt, config))
		fmt.Printf("Availability: %d/%d spots\n",
			selectedClass.BookingCapacity,
			selectedClass.MaxCapacity)
		fmt.Printf("Class ID: %s\n", selectedClass.ID)

		var shouldBook bool
		bookPrompt := &survey.Confirm{
			Message: "Would you like to book this class?",
		}
		if err := survey.AskOne(bookPrompt, &shouldBook); err != nil {
			log.Fatalf("Error during booking confirmation: %v", err)
		}

		if shouldBook {
			needsWaitlist := selectedClass.BookingCapacity <= 0
			if needsWaitlist {
				var useWaitlist bool
				waitlistPrompt := &survey.Confirm{
					Message: "This class is full. Would you like to join the waitlist?",
				}
				if err := survey.AskOne(waitlistPrompt, &useWaitlist); err != nil {
					log.Fatalf("Error during waitlist confirmation: %v", err)
				}
				if !useWaitlist {
					fmt.Println("Booking cancelled.")
					return
				}
			}

			bookingReq := otf_api.CreateBookingRequest{
				ClassID:   selectedClass.ID,
				Confirmed: false,
				Waitlist:  needsWaitlist,
			}

			err := apiClient.BookClass(ctx, bookingReq)
			if err != nil {
				log.Fatalf("Error booking class: %v", err)
			}

			if needsWaitlist {
				fmt.Println("Successfully added to waitlist!")
			} else {
				fmt.Println("Successfully booked the class!")
			}
		} else {
			fmt.Println("Booking cancelled.")
		}
	},
}

func findClassByID(classes []otf_api.StudioClass, id string) (otf_api.StudioClass, bool) {
	for _, c := range classes {
		if c.ID == id {
			return c, true
		}
	}
	return otf_api.StudioClass{}, false
}

// writeJSON writes v as JSON to stdout. Must only be called once per command.
func writeJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		log.Fatalf("error encoding JSON: %v", err)
	}
}

// setupClient creates an authenticated API client.
// It tries a cached token first, then falls back to env var credentials.
func setupClient(ctx context.Context) *otf_api.Client {
	apiClient, err := otf_api.NewClient()
	if err != nil {
		log.Fatalf("Error creating API client: %v", err)
	}

	config, cfgErr := loadConfig()
	tryToken := cfgErr == nil && config.Token != ""

	if tryToken {
		apiClient.SetToken(config.Token)
		apiClient.RefreshToken = config.RefreshToken
		if !apiClient.NeedAuth() {
			return apiClient
		}
	}

	username := os.Getenv("OTF_USERNAME")
	password := os.Getenv("OTF_PASSWORD")
	if username == "" || password == "" {
		log.Fatal("OTF_USERNAME and OTF_PASSWORD environment variables must be set (or cache a token by running a command interactively first)")
	}

	if err := apiClient.Authenticate(ctx, username, password); err != nil {
		log.Fatalf("Error authenticating: %v", err)
	}

	config.Token = apiClient.Token
	config.RefreshToken = apiClient.RefreshToken
	if saveErr := saveConfig(config); saveErr != nil {
		log.Printf("Warning: could not cache token: %v", saveErr)
	}

	return apiClient
}

// Helper to pad or truncate a string to a fixed width
func padOrTruncate(s string, width int) string {
	runes := []rune(s)
	if len(runes) > width {
		return string(runes[:width])
	} else if len(runes) < width {
		return s + strings.Repeat(" ", width-len(runes))
	}
	return s
}

// getConfigPath determines the path for the configuration file.
func getConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user config directory: %w", err)
	}
	cliConfigDir := filepath.Join(configDir, cliDirName)
	if err := os.MkdirAll(cliConfigDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create cli config directory %s: %w", cliConfigDir, err)
	}
	return filepath.Join(cliConfigDir, configFileName), nil
}

// loadConfig loads the CLI configuration from the config file.
func loadConfig() (CLIConfig, error) {
	var config CLIConfig
	configFilePath, err := getConfigPath()
	if err != nil {
		return config, err
	}

	data, err := os.ReadFile(configFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return config, fmt.Errorf("failed to read config file %s: %w", configFilePath, err)
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("failed to unmarshal config data from %s: %w", configFilePath, err)
	}
	return config, nil
}

// saveConfig saves the CLI configuration to the config file.
func saveConfig(config CLIConfig) error {
	configFilePath, err := getConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config to JSON: %w", err)
	}

	if err := os.WriteFile(configFilePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", configFilePath, err)
	}
	return nil
}

// formatTime formats a time.Time value according to the configured timezone
func formatTime(t time.Time, config CLIConfig) string {
	if config.Timezone == "" {
		// Use local timezone if no timezone is configured
		return t.Format("3:04 PM MST")
	}

	// Load the configured timezone
	loc, err := time.LoadLocation(config.Timezone)
	if err != nil {
		log.Printf("Warning: Invalid timezone %s, using local timezone: %v", config.Timezone, err)
		return t.Format("3:04 PM MST")
	}

	// Convert and format the time in the configured timezone
	return t.In(loc).Format("3:04 PM MST")
}

func init() {
	rootCmd.AddCommand(schedulesCmd)
	schedulesCmd.Flags().StringVar(&studioIDs, "studio-ids", "", "Comma-separated list of studio IDs (optional if preferred studios are configured)")
	schedulesCmd.Flags().BoolVar(&scheduleJSON, "json", false, "Output schedule as JSON")
	schedulesCmd.Flags().StringVar(&scheduleClassID, "class-id", "", "Class ID to book (requires --book)")
	schedulesCmd.Flags().BoolVar(&scheduleBook, "book", false, "Book the class specified by --class-id")
	schedulesCmd.Flags().BoolVar(&scheduleYes, "yes", false, "Skip confirmation prompts")

	rootCmd.AddCommand(bookingsCmd)
	bookingsCmd.AddCommand(listBookingsCmd)
	bookingsCmd.AddCommand(cancelBookingCmd)
	listBookingsCmd.Flags().BoolVar(&listBookingsJSON, "json", false, "Output bookings as JSON")
	listBookingsCmd.Flags().StringVar(&listBookingsCancelID, "booking-id", "", "Booking ID to cancel (requires --cancel)")
	listBookingsCmd.Flags().BoolVar(&listBookingsCancelFlag, "cancel", false, "Cancel the booking specified by --booking-id")
	listBookingsCmd.Flags().BoolVar(&listBookingsYes, "yes", false, "Skip cancellation confirmation")
	cancelBookingCmd.Flags().BoolVar(&cancelBookingYes, "yes", false, "Skip confirmation prompt")

	rootCmd.AddCommand(configureCmd)
	configureCmd.AddCommand(configureStudiosCmd)
	configureStudiosCmd.Flags().BoolVar(&configureStudioJSON, "json", false, "Output studio list as JSON")
	configureStudiosCmd.Flags().Float64Var(&configureStudioLat, "lat", 0, "Latitude for studio search")
	configureStudiosCmd.Flags().Float64Var(&configureStudioLong, "long", 0, "Longitude for studio search")
	configureStudiosCmd.Flags().Float64Var(&configureStudioDist, "distance", 10, "Search radius in miles")
	configureStudiosCmd.Flags().BoolVar(&configureStudioSave, "save", false, "Save all found studios as preferred")
	configureCmd.AddCommand(configureTimezoneCmd)
	configureTimezoneCmd.Flags().StringVar(&configureTimezoneFlag, "timezone", "", "Set timezone directly (e.g. America/New_York). Empty string clears it.")
}

func main() {
	_ = godotenv.Load() // optional; env vars may come from the environment

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Whoops. There was an error while executing your CLI '%s'", err)
		os.Exit(1)
	}
}
