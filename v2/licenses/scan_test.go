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

package licenses_test

import (
	"testing"

	"github.com/google/go-licenses/v2/licenses"
	"github.com/stretchr/testify/assert"
)

const DbPath = "../third_party/google/licenseclassifier/licenses"

func TestScan_ThisRepo(t *testing.T) {
	found, err := licenses.ScanDir(
		"..", // repo root
		licenses.ScanDirOptions{
			DbPath: DbPath,
			ExcludePaths: []string{
				// binaries
				"go-licenses",
				"deps/testdata",
				// testdata
				"licenses/testdata",
				// distribution
				"dist",
				// license db
				"third_party/google/licenseclassifier/licenses",
				// notices
				"third_party/NOTICES",
			},
		},
	)
	if err != nil {
		t.Error(err)
	}
	expected := []licenses.File{
		{
			Path:     "third_party/go/runtime/debug/LICENSE",
			Licenses: []licenses.Found{{SpdxId: "BSD-3-Clause", StartLine: 3, EndLine: 27, Confidence: 0.9812206572769953}},
		},
		{
			Path:     "third_party/google/licenseclassifier/LICENSE",
			Licenses: []licenses.Found{{SpdxId: "Apache-2.0", StartLine: 2, EndLine: 175, Confidence: 1}},
		},
	}
	assert.Equal(t, expected, found)
}

func TestScan_DirWithSymlink(t *testing.T) {
	found, err := licenses.ScanDir(
		"testdata/folder-with-symlink",
		licenses.ScanDirOptions{
			DbPath: DbPath,
		},
	)
	if err != nil {
		t.Error(err)
	}
	expected := []licenses.File{}
	assert.Equal(t, expected, found)
}
