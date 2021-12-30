// Copyright 2019 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/csv"
	"os"

	"github.com/google/go-licenses/v2/licenses"
	"github.com/spf13/cobra"
)

var (
	csvCmd = &cobra.Command{
		Use:   "csv <package>",
		Short: "Prints all licenses that apply to a Go package and its dependencies",
		Args:  cobra.MinimumNArgs(1),
		RunE:  csvMain,
		// Avoid usage and errors being printed unnecessarily: https://github.com/spf13/cobra/issues/340
		SilenceUsage:  true,
		SilenceErrors: true,
	}
)

func init() {
	rootCmd.AddCommand(csvCmd)
}

func csvMain(_ *cobra.Command, args []string) error {
	writer := csv.NewWriter(os.Stdout)

	classifier, err := licenses.NewClassifier(confidenceThreshold)
	if err != nil {
		return err
	}

	mods, err := licenses.Modules(classifier, args...)
	if err != nil {
		return err
	}
	for _, mod := range mods {
		for _, license := range mod.Licenses {
			licenseURL := "Unknown"
			if err := writer.Write([]string{mod.Path, licenseURL, license.ID}); err != nil {
				return err
			}
		}
	}
	writer.Flush()
	return writer.Error()
}
