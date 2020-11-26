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
	"go-dynamic-fetch/fetcher"

	"github.com/spf13/cobra"
)

// emailCmd represents the email command
var emailCmd = &cobra.Command{
	Use:   "email",
	Short: "Emails if the desired criteria is met in watch",
	Long:  `This is on of the actions that can be taken for watch - it will send an email from the provided email to the receipient email`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		f := cmd.Flags()

		from, err := f.GetString("from")
		if err != nil {
			return err
		} else if from == "" {
			return fmt.Errorf("Please specify from email address")
		}

		to, err := f.GetString("to")
		if err != nil {
			return err
		} else if to == "" {
			return fmt.Errorf("Please specify to email address")
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return fetcher.EmailContent(cmd)
	},
}

func init() {
	watchCmd.AddCommand(emailCmd)

	emailCmd.Flags().String("subject", fetcher.DefaultSubject, "Subject to be specified")
	emailCmd.Flags().String("from", "", "Email address to send message from")
	emailCmd.Flags().String("to", "", "Email address to send message to")
	emailCmd.Flags().String("sender-password", "", "Password for the from email specified (specify as an environment variable)")
}
