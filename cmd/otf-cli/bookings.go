package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/ammiranda/otf_api"
	"github.com/mgutz/ansi"
	"github.com/spf13/cobra"
)

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

		if listBookingsJSON {
			writeJSON(bookings)
			return
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
