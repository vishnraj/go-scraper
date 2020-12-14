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
	"github.com/spf13/viper"
)

// watchCmd represents the watch command
var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch URL(s) and take an action if criteria is met",
	Long:  `This command provides sub-commands that we can run to take a particular action if the selectors (in the order of URLs specified) are found on the particular web-page (for the timeout set) and it will keep watching for the selectors at the set interval`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		viper.BindPFlags(cmd.Flags())
		urls := viper.GetStringSlice("urls")
		if urls == nil {
			return fmt.Errorf("We require a non-empty comma separated slice of URL(s)")
		}

		selectors := viper.GetStringSlice("wait-selectors")
		if selectors == nil {
			return fmt.Errorf("We require a non-empty comma separated slice of selector(s)")
		}

		if len(urls) != len(selectors) || len(urls) == 0 {
			return fmt.Errorf("Number of URLs and selectors passed in must have the same length and be non-zero")
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("Must call a sub-command of watch")
	},
}

func init() {
	rootCmd.AddCommand(watchCmd)

	watchCmd.PersistentFlags().StringSlice("urls", nil, "All URLs to watch")
	watchCmd.PersistentFlags().StringSlice("wait-selectors", nil, "All selectors, in order of URLs passed in, to wait for")
	watchCmd.PersistentFlags().IntP("interval", "i", fetcher.DefaultInterval, "Interval (in seconds) to wait in between watching a selector")
}
