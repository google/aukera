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

//go:build linux
// +build linux

package auklib

import (
	"fmt"
	"runtime"
	"time"
)

var (
	// DataDir defines app data filesystem location.
	DataDir = "/var/lib/aukera"
	// ConfDir defines configuration JSON filesystem location.
	ConfDir = "/etc/aukera"
	// LogPath defines active log file filesystem location.
	LogPath = "/var/log/aukera.log"

	// MetricSvc sets platform source for metrics.
	MetricSvc = "aukera"
	// MetricRoot sets metric path for all aukera metrics
	MetricRoot = `/aukera/metrics`
)

// ActiveHours retrieves the user/auto-set active hours times.
// Stubbed out on linux.
func ActiveHours() (time.Time, time.Time, error) {
	var t time.Time
	return t, t, fmt.Errorf("ActiveHours: unsupported operating system: %s", runtime.GOOS)
}
