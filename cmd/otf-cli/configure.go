package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/ammiranda/otf_api/otf_api"
	"github.com/spf13/cobra"
)

var (
	configureStudioJSON bool
	configureStudioLat  float64
	configureStudioLong float64
	configureStudioDist float64
	configureStudioSave bool
	configureTimezoneFlag string
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure otf-cli settings",
	Long:  `Provides commands to configure various settings for the otf-cli, such as preferred studios.`,
}

var configureStudiosCmd = &cobra.Command{
	Use:   "studios",
	Short: "Find and save preferred studios",
	Long: `Search for OTF studios by location and save your favorites.

Interactive mode auto-detects your location from your IP, then lets you
select which studios to track. These are used by 'schedules' and 'otf-mcp'.

With --lat, --long, --distance, runs non-interactively.
With --save, saves all found studio IDs as preferred.

Examples:
  otf-cli configure studios                        # interactive (auto-detect IP)
  otf-cli configure studios --lat 40.71 --long -74.00 --distance 10  # search NYC area
  otf-cli configure studios --lat 40.71 --long -74.00 --save          # search & save`,
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
				config = otf_api.CLIConfig{}
			}
			config.PreferredStudioIDs = ids
			if err := saveConfig(config); err != nil {
				log.Fatalf("Error saving configuration: %v", err)
			}
			log.Printf("Saved %d studios as preferred", len(ids))
		}

		if !configureStudioJSON && !configureStudioSave {
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
				config = otf_api.CLIConfig{}
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

var configureTimezoneCmd = &cobra.Command{
	Use:   "timezone",
	Short: "Set your display timezone",
	Long: `Set your preferred timezone for displaying class times.

With --timezone, sets it directly. Use --timezone "" to use system local.

Examples:
  otf-cli configure timezone                    # interactive picker
  otf-cli configure timezone --timezone "America/New_York"
  otf-cli configure timezone --timezone ""`,
	Run: func(cmd *cobra.Command, args []string) {
		config, err := loadConfig()
		if err != nil {
			log.Printf("Warning: Could not load existing config, will create a new one: %v", err)
			config = otf_api.CLIConfig{}
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
