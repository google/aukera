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

// Package auklib contains utility functions and values for Aukera.
package auklib

import (
	"fmt"
	"os"
	"strings"
)

const (
	// ServiceName defines the name of Aukera Windows service.
	ServiceName = "Aukera"
)

// PathExists used for determining if path exists already.
func PathExists(path string) (bool, error) {
	if path == "" {
		return false, fmt.Errorf("PathExists: received empty string to test")
	}

	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// UniqueStrings returns a deduplicated represenation of the passed string slice.
func UniqueStrings(slice []string) []string {
	var unique []string
	m := make(map[string]bool)
	for _, s := range slice {
		if !m[s] {
			m[s] = true
			unique = append(unique, s)
		}
	}
	return unique
}

// ToLowerSlice lowers capitalization of every string in the given slice.
func ToLowerSlice(slice []string) []string {
	var out []string
	for _, s := range slice {
		out = append(out, strings.ToLower(s))
	}
	return out
}
