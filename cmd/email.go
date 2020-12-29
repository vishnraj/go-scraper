/*
Package cmd defines commands
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"

	"github.com/vishnraj/go-scraper/fetcher"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// emailCmd represents the email command
var emailCmd = &cobra.Command{
	Use:   "email",
	Short: "Emails if the desired criteria is met in watch",
	Long:  `This is one of the actions that can be taken for watch - it will send an email from the provided sender email to the receipient email`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		viper.BindPFlags(cmd.Flags())
		from := viper.GetString("from")
		if from == "" {
			return fmt.Errorf("Please specify from email address")
		}

		to := viper.GetString("to")
		if to == "" {
			return fmt.Errorf("Please specify to email address")
		}

		password := viper.GetString("email_password")
		if len(password) == 0 {
			return fmt.Errorf("We require a non-empty sender email password")
		}

		return fetcher.CommonWatchChecks(cmd)
	},
	Run: func(cmd *cobra.Command, args []string) {
		fetcher.EmailContent(cmd)
	},
}

func init() {
	watchCmd.AddCommand(emailCmd)

	emailCmd.Flags().String("subject", fetcher.DefaultSubject, "Subject to be specified")
	emailCmd.Flags().String("from", "", "Email address to send message from")
	emailCmd.Flags().String("to", "", "Email address to send message to")
	emailCmd.Flags().String("email_password", "", "Password for the from email specified (specify as an environment variable)")
}
