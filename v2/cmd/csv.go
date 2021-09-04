// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	configmodule "github.com/google/go-licenses/v2/config"
	"github.com/google/go-licenses/v2/ghutils"
	"github.com/google/go-licenses/v2/gocli"
	"github.com/google/go-licenses/v2/goutils"
	"github.com/google/go-licenses/v2/licenses"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

// csvCmd represents the csv command
var csvCmd = &cobra.Command{
	Use:   "csv <BINARY_PATH>",
	Short: "Generate dependency license csv",
	Long: `Generate licenses csv table for the go binary. The command must
	be run in the go module workdir used to build the binary.
	The tool mainly uses google/licenseclassifier/v2 to get license info.
	There may be false positives. Use it at your own risk.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		binaryPath := args[0]
		err := csvImp(context.Background(), binaryPath)
		if err != nil {
			klog.Exit(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(csvCmd)
}

func csvImp(ctx context.Context, binaryPath string) (err error) {
	config, err := configmodule.Load("")
	if err != nil {
		return err
	}

	if config.Module.LicenseDB.Path == "" {
		config.Module.LicenseDB.Path, err = defaultLicenseDB()
		if err != nil {
			klog.Exit(fmt.Errorf("licenseDB.path is empty, also failed to get defaulut licenseDB path: %w", err))
		}
		klog.V(2).InfoS("Config: use default license DB")
	}
	klog.V(2).InfoS("Config: license DB path", "path", config.Module.LicenseDB.Path)

	metadata, err := gocli.ExtractBinaryMetadata(binaryPath)
	if err != nil {
		return err
	}
	goModules := metadata.Deps
	main, err := mainModule(metadata, config)
	if err != nil {
		return err
	}
	goModules = append([]gocli.Module{*main}, goModules...)
	klog.InfoS("Done: found dependencies", "count", len(goModules))
	if klog.V(3).Enabled() {
		for _, goModule := range goModules {
			klog.InfoS("dependency", "module", goModule.Path, "version", goModule.Version, "Dir", goModule.Dir)
		}
	}
	f := os.Stdout // TODO: support writing to a file directly
	if err != nil {
		return errors.Wrapf(err, "Creating license csv file")
	}
	defer func() {
		closeErr := f.Close()
		if err == nil {
			// When there are no other errors, surface close file error.
			// Otherwise file content may not be flushed to disk successfully.
			err = closeErr
		}
	}()
	_, err = f.WriteString("# Generated by https://github.com/google/go-licenses/v2. DO NOT EDIT.\n")
	if err != nil {
		return err
	}
	licenseCount := 0
	errorCount := 0
	for _, goModule := range goModules {
		report := func(err error, args ...interface{}) {
			errorCount = errorCount + 1
			errorArgs := []interface{}{"module", goModule.Path}
			errorArgs = append(errorArgs, args...)
			klog.ErrorS(err, "Failed", errorArgs...)
		}
		var override configmodule.ModuleOverride
		for _, o := range config.Module.Overrides {
			if o.Name == goModule.Path {
				override = o
			}
		}
		// When override.Version == "", the override apply to any version.
		if override.Version != "" && override.Version != goModule.Version {
			report(fmt.Errorf("override version mismatch: found %s, but override is for %s", goModule.Version, override.Version))
			continue
		}
		if override.Skip {
			klog.InfoS("Skipped", "module", goModule.Path)
			continue
		}
		repo, errGetGithubRepo := goutils.GetGithubRepo(goModule.Path)
		// this is not immediately an error, because we might specify override.License.Url below
		type licenseInfo struct {
			spdxId        string // required
			licensePath   string // optional, required when url is not supplied
			url           string // optional
			subModulePath string // optional
			lineStart     int    // optional
			lineEnd       int    // optional
		}
		hasReportedGetGithubRepoErr := false
		writeLicenseInfo := func(info licenseInfo) error {
			if info.spdxId == "" {
				return fmt.Errorf("failed writeLicenseInfo: info.spdxId required")
			}
			url := info.url
			if url == "" {
				if info.licensePath == "" {
					return fmt.Errorf("failed writeLicenseInfo: info.licensePath required when info.url is empty")
				}
				if repo == nil && !hasReportedGetGithubRepoErr {
					// now we need to use repo, so this becomes a fatal error
					report(errGetGithubRepo)
					hasReportedGetGithubRepoErr = true // only report once
					// when repo == nil, repo.RemoteUrl has fallback behavior to use local path,
					// so keep running to show more information to debug.
				}
				licensePath := info.licensePath
				if info.subModulePath != "" && info.subModulePath != "." {
					licensePath = info.subModulePath + "/" + info.licensePath
				}
				url, err = repo.RemoteUrl(ghutils.RemoteUrlArgs{
					Path:      licensePath,
					Version:   goModule.Version,
					LineStart: info.lineStart,
					LineEnd:   info.lineEnd,
				})
				if err != nil {
					return err
				}
			}
			moduleString := goModule.Path
			if info.subModulePath != "" {
				moduleString = moduleString + "/" + info.subModulePath
			}
			_, err := f.WriteString(fmt.Sprintf(
				"%s, %s, %s\n",
				moduleString,
				url,
				info.spdxId))
			if err != nil {
				return fmt.Errorf("Failed to write string: %w", err)
			}
			licenseCount = licenseCount + 1
			return nil
		}

		if override.License.SpdxId != "" {
			license := override.License
			if license.Path == "" && license.Url == "" {
				report(fmt.Errorf("At least one of override.license.Path and override.license.Url is required"))
				continue
			}
			klog.V(4).InfoS("License overridden", "module", goModule.Path, "version", goModule.Version, "Dir", goModule.Dir)
			klog.V(5).InfoS("Override config", "override", fmt.Sprintf("%+v", override))
			err := writeLicenseInfo(licenseInfo{
				url:         license.Url,
				licensePath: license.Path,
				spdxId:      license.SpdxId,
				lineStart:   license.LineStart,
				lineEnd:     license.LineEnd,
			})
			if err != nil {
				return err
			}
			for _, subModule := range override.SubModules {
				license := subModule.License
				if len(subModule.Path) == 0 || len(license.Path) == 0 || len(license.SpdxId) == 0 {
					report(fmt.Errorf("override.subModule: path, license.path and license.spdxId are required: subModule=%+v", subModule))
					continue
				}
				err := writeLicenseInfo(licenseInfo{
					url:           license.Url,
					licensePath:   license.Path,
					spdxId:        license.SpdxId,
					lineStart:     license.LineStart,
					lineEnd:       license.LineEnd,
					subModulePath: subModule.Path,
				})
				if err != nil {
					return err
				}
			}
			continue
		}

		klog.V(4).InfoS("Scanning", "module", goModule.Path, "version", goModule.Version, "Dir", goModule.Dir)
		fileLicenses, err := licenses.ScanDir(goModule.Dir, licenses.ScanDirOptions{ExcludePaths: override.ExcludePaths, DbPath: config.Module.LicenseDB.Path})
		if err != nil {
			report(err)
			continue
		}
		if len(fileLicenses) == 0 {
			report(errors.Errorf("licenses not found"))
			continue
		}

		for _, file := range fileLicenses {
			spdxIds := make([]string, 0)
			for _, license := range file.Licenses {
				// We need the joinedSpdxId to be deterministic,
				// because we want to verify found licenses are
				// the same as what people have verified manually
				// last time.
				// If we use map[string]bool, we cannot guarantee
				// order.
				// Although slightly inefficient, looping
				// through the array to find whether a license
				// is a new found does guarantee we are appending
				// licenses into the array in a deterministic
				// order.
				found := false
				for _, spdxId := range spdxIds {
					if license.SpdxId == spdxId {
						found = true
					}
				}
				if !found {
					spdxIds = append(spdxIds, license.SpdxId)
				}
			}
			var joinedSpdxId = ""
			for _, spdxId := range spdxIds {
				if joinedSpdxId == "" {
					joinedSpdxId = spdxId
				} else {
					joinedSpdxId = joinedSpdxId + " / " + spdxId
				}
			}
			klog.V(3).InfoS("License", "module", goModule.Path, "SpdxId", joinedSpdxId, "path", filepath.Join(goModule.Dir, file.Path))
			writeLicenseInfo(licenseInfo{
				spdxId:      joinedSpdxId,
				licensePath: file.Path,
			})
			if err != nil {
				return err
			}
		}
	}
	if errorCount > 0 {
		return fmt.Errorf("Failed to scan licenses for %v module(s)", errorCount)
	}
	klog.InfoS("Done: scan licenses of dependencies", "licenseCount", licenseCount, "moduleCount", len(goModules))
	return nil
}

func defaultLicenseDB() (string, error) {
	execDir, err := findExecutable()
	if err != nil {
		return "", fmt.Errorf("findLicenseDB failed: %w", err)
	}
	return filepath.Join(execDir, "licenses"), nil
}

func findExecutable() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("findExecutable failed: %w", err)
	}
	dirPath := filepath.Dir(path)
	return dirPath, nil
}

func mainModule(metadata *gocli.BinaryMetadata, config *configmodule.GoModLicensesConfig) (mod *gocli.Module, err error) {
	defer func() {
		if err != nil {
			// wrap consistent error message
			err = fmt.Errorf("Error getting main module info: %w", err)
		}
	}()
	if metadata == nil {
		return nil, fmt.Errorf("No binary metadata")
	}
	version := "main"
	if config != nil && config.Module.Go.Version != "" {
		version = config.Module.Go.Version
	}
	metadata.Main.Version = version
	return &metadata.Main, nil
}
