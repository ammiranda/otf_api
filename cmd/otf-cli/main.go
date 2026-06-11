package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "0.1.0"

var rootCmd = &cobra.Command{
	Use:     "otf-cli",
	Version: version,
	Short:   "CLI for the OrangeTheory Fitness API",
	Long: `otf-cli lets you browse OTF class schedules, book/cancel classes,
and manage your preferred studios — all from the terminal.

  • Browse schedules across multiple studios with color-coded output
  • Book classes interactively or non-interactively (for scripts)
  • Manage bookings (list, cancel)
  • Search and configure preferred studios by location

See https://github.com/ammiranda/otf-api for more information.`,
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
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Whoops. There was an error while executing your CLI '%s'", err)
		os.Exit(1)
	}
}
