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

	"github.com/vishnraj/go-dynamic-fetch/fetcher"

	"github.com/spf13/cobra"
)

// fetchCmd represents the fetch command
var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Write the HTML content for the URL to stdout",
	Long:  `Fetches all content from the URL in HTML format and writes it to stdout`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		f := cmd.Flags()
		v, err := f.GetString("url")
		if err != nil {
			return err
		} else if v == "" {
			return fmt.Errorf("We require a non-empty URL")
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return fetcher.PrintContent(cmd)
	},
}

func init() {
	rootCmd.AddCommand(fetchCmd)

	fetchCmd.Flags().StringP("url", "u", "", "URL that you are fetching HTML content for")
	fetchCmd.Flags().String("wait-selector", "", "Selector for element to wait for - if not specified we do not wait and just dump static elements")
	fetchCmd.Flags().String("text-selector", "", "Gets and prints text for the desired selector and if not specified dump all content retrieved")
}
