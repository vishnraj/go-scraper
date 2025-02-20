package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vishnraj/go-scraper/fetcher"
)

// discordCmd represents the discord command
var discordCmd = &cobra.Command{
	Use:   "discord",
	Short: "Sends Discord notifications if the desired criteria is met in watch",
	Long:  `This subcommand sends notifications to a Discord channel via a webhook`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		viper.BindPFlags(cmd.Flags())
		webhook := viper.GetString("webhook")
		if webhook == "" {
			return fmt.Errorf("please specify a Discord webhook URL using --webhook")
		}
		username := viper.GetString("discord_username")
		if username == "" {
			return fmt.Errorf("please specify a Discord username using --discord_username")
		}
		return fetcher.CommonWatchChecks(cmd)
	},
	Run: func(cmd *cobra.Command, args []string) {
		fetcher.DiscordContent(cmd)
	},
}

func init() {
	watchCmd.AddCommand(discordCmd)

	discordCmd.Flags().String("webhook", "", "Discord webhook URL to send notifications to")
	discordCmd.Flags().String("discord_username", "", "Username to display in Discord notifications")
}
