// Copyright 2018 Google LLC
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

package auklib

import (
	"os"
	"runtime"
	"testing"
)

type pathTest struct {
	desc   string
	path   string
	expect bool
}

func TestPathExists(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "")
	defer os.RemoveAll(tempDir)
	if err != nil {
		t.Fatalf("error creating temp directory: %v", err)
	}
	tests := []pathTest{
		{"generated test dir", tempDir, true},
		{"made up path", "/probably/a/made/up/path/to/nothing", false},
	}
	if runtime.GOOS == "windows" {
		tests = append(tests, pathTest{"windows root dir", `C:\`, true})
	}
	for _, p := range tests {
		b, err := PathExists(p.path)
		if b != p.expect {
			t.Errorf("TestPathExists(%q) should be: %t, was: %t", p.desc, p.expect, b)
		}
		if err != nil {
			t.Errorf("TestPathExists(%q) returned error: %v", p.desc, err)
		}
	}
}

func TestEmptyPath(t *testing.T) {
	empty := pathTest{"empty path", "", false}
	b, err := PathExists(empty.path)
	if err == nil {
		t.Errorf("TestEmptyPath(%q) did not result in error output.", empty.desc)
	}
	if b != empty.expect {
		t.Errorf("TestEmptyPath(%q) returned %t", empty.desc, b)
	}
}
