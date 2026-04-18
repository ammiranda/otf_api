package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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

var configureStudiosCmd = &cobra.Command{
	Use:   "studios",
	Short: "Configure preferred OTF studios",
	Long: `Allows you to search for OTF studios by location and save your preferred ones. 
These saved studios will be used by the 'schedules' command if no --studio-ids are specified.`,
	Run: func(cmd *cobra.Command, args []string) {
		apiClient, err := authenticateClient()
		if err != nil {
			log.Fatal(err)
		}

		location, err := getLocation()
		if err != nil {
			log.Fatalf("Error getting location: %v", err)
		}

		distance, err := promptForDistance()
		if err != nil {
			log.Fatalf("Error getting distance: %v", err)
		}

		log.Printf("Using location %s: %.6f, %.6f", location.Source, location.Lat, location.Long)
		log.Println("Fetching studios near you...")

		ctx := context.Background()
		studioListResponse, err := apiClient.ListStudios(ctx, location.Lat, location.Long, distance)
		if err != nil {
			log.Fatalf("Error fetching studios: %v", err)
		}

		selectedStudioIDs, err := selectStudios(studioListResponse.Data.Data)
		if err != nil {
			log.Fatalf("Error selecting studios: %v", err)
		}

		if err := saveStudioPreferences(selectedStudioIDs); err != nil {
			log.Fatalf("Error saving preferences: %v", err)
		}
	},
}

var configureTimezoneCmd = &cobra.Command{
	Use:   "timezone",
	Short: "Configure your preferred timezone",
	Long:  `Set your preferred timezone for displaying class times. If not set, the system's local timezone will be used.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get list of common timezones
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

		// Load existing config
		config, err := loadConfig()
		if err != nil {
			log.Printf("Warning: Could not load existing config, will create a new one: %v", err)
			config = CLIConfig{}
		}

		// If timezone is already set, add it to the list if it's not there
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

		// Add option to use system timezone
		timezones = append(timezones, "System Local Timezone")

		// Prompt for timezone selection
		var selectedTimezone string
		prompt := &survey.Select{
			Message: "Select your preferred timezone:",
			Options: timezones,
			// Only set default if it exists in the options
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

		// If "System Local Timezone" is selected, clear the timezone setting
		if selectedTimezone == "System Local Timezone" {
			config.Timezone = ""
		} else {
			config.Timezone = selectedTimezone
		}

		// Save the configuration
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

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with your OTF credentials",
	Long:  `Interactive login to authenticate with your OrangeTheory Fitness account. Prompts for username and password, then stores the token for future use.`,
	Run: func(cmd *cobra.Command, args []string) {
		username, password := "", ""

		prompts := []*survey.Question{
			{
				Name:   "username",
				Prompt: &survey.Input{Message: "Username:"},
				Validate: func(val interface{}) error {
					if str, ok := val.(string); !ok || str == "" {
						return fmt.Errorf("username is required")
					}
					return nil
				},
			},
			{
				Name:   "password",
				Prompt: &survey.Password{Message: "Password:"},
				Validate: func(val interface{}) error {
					if str, ok := val.(string); !ok || str == "" {
						return fmt.Errorf("password is required")
					}
					return nil
				},
			},
		}

		answers := struct {
			Username string
			Password string
		}{}
		if err := survey.Ask(prompts, &answers); err != nil {
			log.Fatalf("Error during login prompts: %v", err)
		}
		username = answers.Username
		password = answers.Password

		apiClient, err := otf_api.NewClient()
		if err != nil {
			log.Fatalf("Error creating API client: %v", err)
		}

		ctx := context.Background()
		if authErr := apiClient.Authenticate(ctx, username, password); authErr != nil {
			log.Fatalf("Error authenticating: %v", authErr)
		}

		config, err := loadConfig()
		if err != nil {
			config = CLIConfig{}
		}
		config.Token = apiClient.Token
		if err := saveConfig(config); err != nil {
			log.Fatalf("Error saving token: %v", err)
		}

		fmt.Println("Successfully authenticated! Token saved.")
	},
}

var bookingsCmd = &cobra.Command{
	Use:   "bookings",
	Short: "Manage your OTF bookings",
	Long:  `Commands to list and cancel your OrangeTheory Fitness bookings.`,
}

var listBookingsCmd = &cobra.Command{
	Use:   "list",
	Short: "List your current bookings",
	Long:  `Lists all your current and upcoming OrangeTheory Fitness bookings.`,
	Run: func(cmd *cobra.Command, args []string) {
		apiClient, err := authenticateClient()
		if err != nil {
			log.Fatal(err)
		}

		ctx := context.Background()
		startsAfter := time.Now().Truncate(24 * time.Hour)
		endsBefore := time.Now().AddDate(0, 0, 60)

		bookings, err := apiClient.GetBookings(ctx, startsAfter, endsBefore, true)
		if err != nil {
			log.Fatalf("Error fetching bookings: %v", err)
		}

		if len(bookings) == 0 {
			fmt.Println("No bookings found.")
			return
		}

		config, err := loadConfig()
		if err != nil {
			log.Printf("Warning: Could not load configuration: %v", err)
			config = CLIConfig{}
		}

		activeBookings := filterActiveBookings(bookings)
		if len(activeBookings) == 0 {
			fmt.Println("No active bookings found.")
			return
		}

		selectedBooking, err := selectBookingToCancel(activeBookings, config)
		if err != nil {
			log.Fatalf("Error selecting booking: %v", err)
		}

		if selectedBooking == nil {
			displayAllBookings(bookings, config)
			return
		}

		if err := confirmAndCancelBooking(apiClient, selectedBooking, config); err != nil {
			log.Fatalf("Error canceling booking: %v", err)
		}
	},
}

var cancelBookingCmd = &cobra.Command{
	Use:   "cancel [booking-id]",
	Short: "Cancel a booking",
	Long:  `Cancel a booking by providing the booking ID. Use 'otf-cli bookings list' to see your booking IDs.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		bookingID := args[0]
		username := getEnvVar("OTF_USERNAME")
		password := getEnvVar("OTF_PASSWORD")

		if username == "" || password == "" {
			log.Fatal("Error: OTF_USERNAME and OTF_PASSWORD environment variables must be set.")
		}

		apiClient, err := otf_api.NewClient()
		if err != nil {
			log.Fatalf("Error creating API client: %v", err)
		}

		ctx := context.Background()
		if authErr := apiClient.Authenticate(ctx, username, password); authErr != nil {
			log.Fatalf("Error authenticating: %v", authErr)
		}

		// Confirm cancellation
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

		err = apiClient.CancelBooking(ctx, bookingID)
		if err != nil {
			log.Fatalf("Error canceling booking: %v", err)
		}

		fmt.Printf("Successfully canceled booking %s\n", bookingID)
	},
}

var schedulesCmd = &cobra.Command{
	Use:   "schedules",
	Short: "Fetch studio schedules",
	Long:  `Fetches schedules for the specified studio IDs. Requires OTF_USERNAME, OTF_PASSWORD, and OTF_CLIENT_ID environment variables. If --studio-ids is not provided, it will try to use saved preferred studios.`,
	Run: func(cmd *cobra.Command, args []string) {
		username := getEnvVar("OTF_USERNAME")
		password := getEnvVar("OTF_PASSWORD")
		clientID := getEnvVar("OTF_CLIENT_ID") // Keep this for explicitness, though Authenticate also gets it

		if username == "" || password == "" || clientID == "" {
			log.Fatal("Error: OTF_USERNAME, OTF_PASSWORD, and OTF_CLIENT_ID environment variables must be set.")
		}

		var idsToFetch []string

		if studioIDs != "" { // Flag is provided
			idsToFetch = strings.Split(studioIDs, ",")
		} else {
			// Flag not provided, try to load from config
			config, err := loadConfig()
			if err != nil {
				log.Fatalf("Error loading configuration to get preferred studios: %v. Please run 'otf-cli configure studios' or provide --studio-ids.", err)
			}
			if len(config.PreferredStudioIDs) > 0 {
				idsToFetch = config.PreferredStudioIDs
				log.Printf("Using preferred studio IDs from configuration: %s", strings.Join(idsToFetch, ", "))
			} else {
				log.Fatal("Error: No studio IDs provided via --studio-ids flag and no preferred studios found in configuration. Please run 'otf-cli configure studios' or provide the --studio-ids flag.")
			}
		}

		if len(idsToFetch) == 0 {
			log.Fatal("Error: No studio IDs to fetch. This should not happen if logic above is correct.") // Should be caught by earlier checks
		}

		apiClient, err := otf_api.NewClient()
		if err != nil {
			log.Fatalf("Error creating API client: %v", err)
		}

		ctx := context.Background()
		authErr := apiClient.Authenticate(ctx, username, password)
		if authErr != nil {
			log.Fatalf("Error authenticating: %v", authErr)
		}

		schedules, err := apiClient.GetStudiosSchedules(ctx, idsToFetch)
		if err != nil {
			log.Fatalf("Error fetching schedules: %v", err)
		}

		if len(schedules.Items) == 0 {
			log.Println("No classes found for the selected studios.")
			return
		}

		// Load config for timezone
		config, err := loadConfig()
		if err != nil {
			log.Printf("Warning: Could not load configuration: %v", err)
			config = CLIConfig{}
		}

		// Prepare class options for selection
		classOptions := []string{}
		classMap := make(map[string]otf_api.StudioClass)

		// Define a list of color names supported by ansi.Color
		studioColors := []string{"red", "green", "yellow", "blue", "magenta", "cyan", "white"}
		studioColorMap := make(map[string]string) // studioID -> color name
		colorIdx := 0

		// Detect terminal width and set breakpoints
		termWidth := 80 // default fallback
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

		// First, collect max widths for each column (class name, start time, end time)
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

		// Add a minimum space between columns
		minSpace := 2
		// Studio column will use the rest of the width
		studioColStart := maxClassName + minSpace + maxStartTime + minSpace + maxEndTime + minSpace
		studioColWidth := entryWidth - studioColStart
		if studioColWidth < 10 {
			studioColWidth = 10 // fallback to avoid negative/too small
		}

		// Filter out canceled classes and sort by start time
		activeClasses := make([]otf_api.StudioClass, 0, len(schedules.Items))
		for _, class := range schedules.Items {
			if !class.Canceled {
				activeClasses = append(activeClasses, class)
			}
		}

		// Sort classes by timezone-converted start time to ensure proper day grouping
		sort.SliceStable(activeClasses, func(i, j int) bool {
			var timeI, timeJ time.Time
			if config.Timezone != "" {
				if loc, err := time.LoadLocation(config.Timezone); err == nil {
					timeI = activeClasses[i].StartsAt.In(loc)
					timeJ = activeClasses[j].StartsAt.In(loc)
				} else {
					timeI = activeClasses[i].StartsAt
					timeJ = activeClasses[j].StartsAt
				}
			} else {
				timeI = activeClasses[i].StartsAt
				timeJ = activeClasses[j].StartsAt
			}
			return timeI.Before(timeJ)
		})

		// Group classes by day
		lastDay := ""
		for _, class := range activeClasses {

			// Assign a color to the studio if not already assigned
			studioID := class.Studio.ID
			studioName := class.Studio.Name
			colorName, ok := studioColorMap[studioID]
			if !ok {
				colorName = studioColors[colorIdx%len(studioColors)]
				studioColorMap[studioID] = colorName
				colorIdx++
			}

			// Convert class time to the configured timezone first
			var displayTime time.Time
			if config.Timezone != "" {
				if loc, err := time.LoadLocation(config.Timezone); err == nil {
					displayTime = class.StartsAt.In(loc)
				} else {
					log.Printf("Warning: Invalid timezone %s, using original time: %v", config.Timezone, err)
					displayTime = class.StartsAt
				}
			} else {
				displayTime = class.StartsAt
			}

			// Format the class time using the converted time
			startTime := formatTime(class.StartsAt, config)
			endTime := formatTime(class.EndsAt, config)

			// Get the day string using the same converted time for consistency
			classDay := displayTime.Format("Mon Jan 2")

			// Debug logging to help identify timezone issues (only if DEBUG env var is set)
			if getEnvVar("DEBUG") != "" {
				log.Printf("DEBUG: Class %s - Original: %s, Display: %s, Day: %s",
					class.Name,
					class.StartsAt.Format("2006-01-02 15:04:05 MST"),
					displayTime.Format("2006-01-02 15:04:05 MST"),
					classDay)
			}

			// Insert day header if this is a new day
			if classDay != lastDay {
				header := fmt.Sprintf("─── %s ───", classDay)
				classOptions = append(classOptions, header)
				lastDay = classDay
			}

			// Color the studio name only
			coloredStudio := ansi.Color(studioName, colorName)

			// Pad each column to its max width
			classNameCol := padOrTruncate(class.Name, maxClassName)
			startTimeCol := padOrTruncate(startTime, maxStartTime)
			endTimeCol := padOrTruncate(endTime, maxEndTime)
			studioCol := padOrTruncate(coloredStudio, studioColWidth)

			// Create a display string with aligned columns
			displayStr := fmt.Sprintf("%s%s%s%s%s%s%s",
				classNameCol, strings.Repeat(" ", minSpace),
				startTimeCol, strings.Repeat(" ", minSpace),
				endTimeCol, strings.Repeat(" ", minSpace),
				studioCol,
			)

			// Truncate to entryWidth if needed
			displayStr = padOrTruncate(displayStr, entryWidth)

			classOptions = append(classOptions, displayStr)
			classMap[displayStr] = class
		}

		if len(classOptions) == 0 {
			log.Println("No available classes found for the selected studios.")
			return
		}

		// Prompt for class selection
		var selectedClassDisplay string
		prompt := &survey.Select{
			Message:  "Select a class to book:",
			Options:  classOptions,
			PageSize: 15,
		}
		if err := survey.AskOne(prompt, &selectedClassDisplay); err != nil {
			log.Fatalf("Error during class selection: %v", err)
		}

		// Skip header lines - check if user selected a day header
		if strings.HasPrefix(selectedClassDisplay, "─── ") && strings.HasSuffix(selectedClassDisplay, " ───") {
			log.Fatal("Error: Cannot book a day header. Please select an actual class.")
		}

		selectedClass, ok := classMap[selectedClassDisplay]
		if !ok {
			log.Fatal("Error: Selected class not found in class map")
		}

		// Display selected class details
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

		// Ask if user wants to book the class
		var shouldBook bool
		bookPrompt := &survey.Confirm{
			Message: "Would you like to book this class?",
		}
		if err := survey.AskOne(bookPrompt, &shouldBook); err != nil {
			log.Fatalf("Error during booking confirmation: %v", err)
		}

		if shouldBook {
			// Check if class is full and needs waitlist
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

			// Use the actual format from Charles Proxy capture
			bookingReq := otf_api.CreateBookingRequest{
				ClassID:   selectedClass.ID,
				Confirmed: false,
				Waitlist:  needsWaitlist,
			}

			// Attempt to book the class
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

func getEnvVar(key string) string {
	val := os.Getenv(key)
	return val
}

// authenticateClient creates and authenticates an API client
func authenticateClient() (*otf_api.Client, error) {
	apiClient, err := otf_api.NewClient()
	if err != nil {
		return nil, fmt.Errorf("error creating API client: %v", err)
	}

	config, err := loadConfig()
	if err == nil && config.Token != "" {
		apiClient.SetToken(config.Token)
		return apiClient, nil
	}

	username := getEnvVar("OTF_USERNAME")
	password := getEnvVar("OTF_PASSWORD")

	if username == "" || password == "" {
		return nil, fmt.Errorf("not authenticated. Run 'otf-cli login' or set OTF_USERNAME and OTF_PASSWORD environment variables")
	}

	ctx := context.Background()
	if err := apiClient.Authenticate(ctx, username, password); err != nil {
		return nil, fmt.Errorf("error authenticating: %v", err)
	}

	return apiClient, nil
}

// LocationResult holds location information
type LocationResult struct {
	Lat    float64
	Long   float64
	Source string
}

// getLocationFromIP tries to detect location from IP
func getLocationFromIP() *LocationResult {
	resp, err := http.Get("http://ip-api.com/json/")
	if err != nil {
		return nil
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("error closing response body: %v", err)
		}
	}()

	var location IPLocation
	if err := json.NewDecoder(resp.Body).Decode(&location); err != nil {
		return nil
	}

	return &LocationResult{
		Lat:  location.Lat,
		Long: location.Lon,
		Source: fmt.Sprintf("detected from your IP in %s, %s, %s",
			location.City, location.Region, location.Country),
	}
}

// promptForManualLocation asks user to enter coordinates manually
func promptForManualLocation() (*LocationResult, error) {
	locationQs := []*survey.Question{
		{Name: "latitude", Prompt: &survey.Input{Message: "Enter your latitude (e.g., 40.7128):"}, Validate: survey.Required},
		{Name: "longitude", Prompt: &survey.Input{Message: "Enter your longitude (e.g., -74.0060):"}, Validate: survey.Required},
	}
	locationAnswers := struct {
		Latitude  string `survey:"latitude"`
		Longitude string `survey:"longitude"`
	}{}

	if err := survey.Ask(locationQs, &locationAnswers); err != nil {
		return nil, fmt.Errorf("error getting location input: %v", err)
	}

	lat, errLat := strconv.ParseFloat(locationAnswers.Latitude, 64)
	long, errLong := strconv.ParseFloat(locationAnswers.Longitude, 64)

	if errLat != nil || errLong != nil {
		return nil, fmt.Errorf("invalid numeric input for latitude or longitude")
	}

	return &LocationResult{
		Lat:    lat,
		Long:   long,
		Source: "manually entered",
	}, nil
}

// getLocation gets location either from IP detection or manual input
func getLocation() (*LocationResult, error) {
	// Try IP detection first
	if result := getLocationFromIP(); result != nil {
		return result, nil
	}

	log.Println("Warning: Could not detect location from IP")
	return promptForManualLocation()
}

// promptForDistance asks user for search distance
func promptForDistance() (float64, error) {
	distanceQs := []*survey.Question{
		{Name: "distance", Prompt: &survey.Input{Message: "Enter search distance in miles (e.g., 10):"}, Validate: survey.Required},
	}
	distanceAnswers := struct {
		Distance string `survey:"distance"`
	}{}

	if err := survey.Ask(distanceQs, &distanceAnswers); err != nil {
		return 0, fmt.Errorf("error getting distance input: %v", err)
	}

	dist, err := strconv.ParseFloat(distanceAnswers.Distance, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid numeric input for distance")
	}

	return dist, nil
}

// selectStudios handles studio selection process
func selectStudios(studios []otf_api.Studio) ([]string, error) {
	if len(studios) == 0 {
		return nil, fmt.Errorf("no studios found for the given location and distance")
	}

	studioOptions := []string{}
	studioMap := make(map[string]string)
	for _, studio := range studios {
		displayName := fmt.Sprintf("%s (ID: %s, %.2f miles)", studio.StudioName, studio.StudioUUID, studio.Distance)
		studioOptions = append(studioOptions, displayName)
		studioMap[displayName] = studio.StudioUUID
	}

	selectedDisplayNames := []string{}
	prompt := &survey.MultiSelect{
		Message:  "Select your preferred studios (use space to select, enter to confirm):",
		Options:  studioOptions,
		PageSize: 15,
	}
	if err := survey.AskOne(prompt, &selectedDisplayNames); err != nil {
		return nil, fmt.Errorf("error during studio selection: %v", err)
	}

	selectedStudioIDs := []string{}
	for _, displayName := range selectedDisplayNames {
		if id, ok := studioMap[displayName]; ok {
			selectedStudioIDs = append(selectedStudioIDs, id)
		}
	}

	return selectedStudioIDs, nil
}

// saveStudioPreferences saves selected studio IDs to config
func saveStudioPreferences(studioIDs []string) error {
	config, err := loadConfig()
	if err != nil {
		log.Printf("Warning: Could not load existing config, will create a new one: %v", err)
		config = CLIConfig{}
	}

	config.PreferredStudioIDs = studioIDs
	if err := saveConfig(config); err != nil {
		return fmt.Errorf("error saving configuration: %v", err)
	}

	if len(studioIDs) > 0 {
		log.Printf("Preferred studios saved: %s", strings.Join(studioIDs, ", "))
	} else {
		log.Println("No studios selected. Preferred studios configuration remains unchanged or empty.")
	}

	return nil
}

// filterActiveBookings returns only non-canceled bookings
func filterActiveBookings(bookings []otf_api.BookingRequest) []otf_api.BookingRequest {
	activeBookings := []otf_api.BookingRequest{}
	for _, booking := range bookings {
		if !booking.Canceled {
			activeBookings = append(activeBookings, booking)
		}
	}
	return activeBookings
}

// createBookingSelectionOptions creates display options for booking selection
func createBookingSelectionOptions(bookings []otf_api.BookingRequest, config CLIConfig) ([]string, map[string]otf_api.BookingRequest) {
	bookingOptions := []string{}
	bookingMap := make(map[string]otf_api.BookingRequest)

	for _, booking := range bookings {
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

	return bookingOptions, bookingMap
}

// displayAllBookings shows all bookings grouped by day
func displayAllBookings(bookings []otf_api.BookingRequest, config CLIConfig) {
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
}

// selectBookingToCancel prompts user to select a booking for cancellation
func selectBookingToCancel(activeBookings []otf_api.BookingRequest, config CLIConfig) (*otf_api.BookingRequest, error) {
	bookingOptions, bookingMap := createBookingSelectionOptions(activeBookings, config)
	bookingOptions = append(bookingOptions, "Just view bookings (no action)")

	var selectedBookingDisplay string
	prompt := &survey.Select{
		Message:  "Select a booking to cancel (or just view):",
		Options:  bookingOptions,
		PageSize: 15,
	}
	if err := survey.AskOne(prompt, &selectedBookingDisplay); err != nil {
		return nil, fmt.Errorf("error during booking selection: %v", err)
	}

	if selectedBookingDisplay == "Just view bookings (no action)" {
		return nil, nil // Signal to just view bookings
	}

	selectedBooking, ok := bookingMap[selectedBookingDisplay]
	if !ok {
		return nil, fmt.Errorf("selected booking not found")
	}

	return &selectedBooking, nil
}

// confirmAndCancelBooking displays booking details and confirms cancellation
func confirmAndCancelBooking(apiClient *otf_api.Client, booking *otf_api.BookingRequest, config CLIConfig) error {
	classTime, _ := time.Parse(time.RFC3339, booking.Class.StartsAt)
	fmt.Printf("\nSelected Booking:\n")
	fmt.Printf("Class: %s\n", booking.Class.Name)
	fmt.Printf("Studio: %s\n", booking.Class.Studio.Name)
	fmt.Printf("Time: %s\n", formatTime(classTime, config))
	fmt.Printf("Booking ID: %s\n", booking.ID)

	var shouldCancel bool
	cancelPrompt := &survey.Confirm{
		Message: "Are you sure you want to cancel this booking?",
	}
	if err := survey.AskOne(cancelPrompt, &shouldCancel); err != nil {
		return fmt.Errorf("error during cancellation confirmation: %v", err)
	}

	if !shouldCancel {
		fmt.Println("Cancellation aborted.")
		return nil
	}

	ctx := context.Background()
	if err := apiClient.CancelBooking(ctx, booking.ID); err != nil {
		return fmt.Errorf("error canceling booking: %v", err)
	}

	fmt.Printf("Successfully canceled booking for %s at %s\n",
		booking.Class.Name,
		booking.Class.Studio.Name)

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

	// Add bookings commands
	rootCmd.AddCommand(bookingsCmd)
	bookingsCmd.AddCommand(listBookingsCmd)
	bookingsCmd.AddCommand(cancelBookingCmd)

	// Add configure commands
	rootCmd.AddCommand(configureCmd)
	configureCmd.AddCommand(configureStudiosCmd)
	configureCmd.AddCommand(configureTimezoneCmd)

	// Add login command
	rootCmd.AddCommand(loginCmd)
}

func main() {
	// Load .env file. Errors are ignored if .env doesn't exist.
	err := godotenv.Load()
	if err != nil {
		log.Printf("Error loading .env file: %v", err)
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Whoops. There was an error while executing your CLI '%s'", err)
		os.Exit(1)
	}
}
