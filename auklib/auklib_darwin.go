// Copyright 2022 Google LLC
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

//go:build darwin
// +build darwin

package auklib

var (
	// DataDir defines app data filesystem location.
	DataDir = "/var/lib/aukera"
	// ConfDir defines configuration JSON filesystem location.
	ConfDir = "/var/lib/aukera/conf.d"
	// LogPath defines active log file filesystem location.
	LogPath = "/var/log/aukera.log"

	// MetricRoot sets metric path for all aukera metrics
	MetricRoot = `/aukera/metrics`
	// MetricSvc sets platform source for metrics.
	MetricSvc = "darwin"
)
