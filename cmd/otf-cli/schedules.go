package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/ammiranda/otf-api"
	"github.com/mgutz/ansi"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	studioIDs      string
	scheduleJSON   bool
	scheduleClassID string
	scheduleBook   bool
	scheduleYes    bool
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
