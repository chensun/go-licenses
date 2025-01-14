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

package config_test

import (
	"testing"

	"github.com/google/go-licenses/v2/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_DefaultPath(t *testing.T) {
	_, err := config.Load("")
	// default path is current folder, so it doesn't exist
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "no such file or directory")
}

func TestLoadConfig_SpecifiedPath(t *testing.T) {
	loaded, err := config.Load("testdata/1.yaml")
	require.Nil(t, err)
	assert.Equal(t, ".cache/licenses", loaded.Module.LicenseDB.Path)
	assert.Equal(t, "github.com/google/go-licenses/v2", loaded.Module.Go.Module)
	expected := []config.ModuleOverride{
		{
			Name:         "github.com/google/go-licenses/v2",
			Version:      "",
			License:      config.LicenseOverride{Path: "LICENSE", SpdxId: "Apache-2.0", Url: "https://github.com/google/go-licenses/v2/dummy-url"},
			ExcludePaths: []string{"go-licenses"},
		}, {
			Name:    "github.com/aws/aws-sdk-go",
			Version: "v1.36.1",
			License: config.LicenseOverride{Path: "LICENSE.txt", SpdxId: "Apache-2.0"},
			SubModules: []config.SubModule{
				{
					Path:    "internal/sync/singleflight",
					License: config.LicenseOverride{Path: "LICENSE", SpdxId: "BSD-3-Clause"},
				},
			},
		},
		{
			Name:         "github.com/google/licenseclassifier",
			ExcludePaths: []string{"licenses"},
		},
		{
			Name:    "cloud.google.com/go",
			Version: "v0.72.0",
			License: config.LicenseOverride{
				Path:   "LICENSE",
				SpdxId: "Apache-2.0",
			},
			SubModules: []config.SubModule{
				{
					Path: "cmd/go-cloud-debug-agent/internal/debug/elf",
					License: config.LicenseOverride{
						Path:      "elf.go",
						SpdxId:    "BSD-2-Clause",
						LineStart: 1,
						LineEnd:   43,
					},
				}, {
					Path:    "third_party/pkgsite",
					License: config.LicenseOverride{Path: "LICENSE", SpdxId: "BSD-3-Clause"},
				},
			},
		},
	}
	assert.Equal(t, expected, loaded.Module.Overrides)
	assert.Equal(t, config.LicensesConfig{
		Types: config.LicenseTypes{
			Overrides: []config.LicenseTypeOverride{{
				SpdxId: "blessing", Type: "unencumbered",
			}},
		},
	}, loaded.Licenses)
}

func TestLoadConfig_PathNotExist(t *testing.T) {
	_, err := config.Load("file-not-exist")
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "no such file or directory")
}

func TestLoadConfig_ErrorOnTypo(t *testing.T) {
	// there is a typo in the config yaml, so we have unknown fields
	_, err := config.Load("testdata/typo.yaml")
	require.NotNil(t, err, "should report error when config has unknown fields")
}
