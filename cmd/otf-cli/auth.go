package main

import (
	"context"
	"log"

	"github.com/AlecAivazis/survey/v2"
	"github.com/ammiranda/otf_api/otf_api"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with your OTF account",
	Long: `Prompts for your OTF credentials and stores them securely in the system keychain.

Run this once before using other commands. After authentication, your login
session is cached so you won't need to re-enter credentials (tokens refresh
automatically).

Examples:
  otf-cli auth             # interactive prompt
  otf-cli auth --check     # verify current auth status`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		var username string
		promptUsername := &survey.Input{
			Message: "OTF account email:",
		}
		if err := survey.AskOne(promptUsername, &username, survey.WithValidator(survey.Required)); err != nil {
			log.Fatalf("Error reading username: %v", err)
		}

		var password string
		promptPassword := &survey.Password{
			Message: "Password:",
		}
		if err := survey.AskOne(promptPassword, &password, survey.WithValidator(survey.Required)); err != nil {
			log.Fatalf("Error reading password: %v", err)
		}

		log.Println("Authenticating...")
		apiClient := otf_api.NewClient()
		if err := apiClient.Authenticate(ctx, username, password); err != nil {
			log.Fatalf("Authentication failed: %v", err)
		}
		log.Println("Authenticated successfully!")

		config, err := loadConfig()
		if err != nil {
			config = otf_api.CLIConfig{}
		}
		config.Username = username
		config.Password = password
		config.Token = apiClient.Token
		config.RefreshToken = apiClient.RefreshToken

		if err := saveConfig(config); err != nil {
			log.Fatalf("Failed to cache credentials: %v", err)
		}
		log.Println("Credentials saved successfully.")
	},
}

func init() {
	rootCmd.AddCommand(authCmd)
}
