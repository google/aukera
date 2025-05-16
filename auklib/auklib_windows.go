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

//go:build windows
// +build windows

package auklib

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows/registry"
)

var (
	// DataDir defines app data filesystem location.
	DataDir = filepath.Join(os.Getenv("ProgramData"), "Aukera")
	// ConfDir defines configuration JSON filesystem location.
	ConfDir = filepath.Join(DataDir, "conf")
	// LogPath defines active log file filesystem location.
	LogPath = filepath.Join(DataDir, "aukera.log")

	// MetricRoot sets metric path for all aukera metrics
	MetricRoot = `/aukera/metrics`
	// MetricSvc sets platform source for metrics.
	MetricSvc = "aukera"

	activeHoursStart, activeHoursEnd uint64
	activeStartTime, activeEndTime   time.Time
)

const (
	activeHoursPath = `SOFTWARE\Microsoft\WindowsUpdate\UX\Settings\`
)

// ActiveHours retrieves the user/auto-set active hours times from the registry.
// Returns the start and end times of the active hours window, respectively.
func ActiveHours() (time.Time, time.Time, error) {

	k, err := registry.OpenKey(registry.LOCAL_MACHINE, activeHoursPath, registry.ALL_ACCESS)
	if err != nil {
		return activeStartTime, activeEndTime, err
	}
	defer k.Close()

	activeHoursStart, _, err = k.GetIntegerValue("ActiveHoursStart")
	if err != nil && err != registry.ErrNotExist {
		return activeStartTime, activeEndTime, fmt.Errorf("unable to get active hours start time: %v", err)
	}

	if err == registry.ErrNotExist {
		// If the ActiveHoursStart value does not exist, set it to the Microsoft default of 8:00 AM.
		if err := k.SetDWordValue("ActiveHoursStart", 8); err != nil {
			return activeStartTime, activeEndTime, fmt.Errorf("ActiveHoursStart not found and unable to set default active hours start time: %v", err)
		}
		activeHoursStart = 8
	}

	now := time.Now()
	activeStartTime = time.Date(now.Year(), now.Month(), now.Day(), int(activeHoursStart), 0, 0, 0, now.Location())

	activeHoursEnd, _, err = k.GetIntegerValue("ActiveHoursEnd")
	if err != nil && err != registry.ErrNotExist {
		return activeStartTime, activeEndTime, fmt.Errorf("unable to get active hours end time: %v", err)
	}

	if err != nil && err == registry.ErrNotExist {
		// If the ActiveHoursEnd value does not exist, set it to the Microsoft default of 5:00 PM.
		if err := k.SetDWordValue("ActiveHoursEnd", 17); err != nil {
			return activeStartTime, activeEndTime, fmt.Errorf("ActiveHoursEnd not found and unable to set default active hours end time: %v", err)
		}
		activeHoursEnd = 17
	}

	var day int
	if int(activeHoursEnd) < activeStartTime.Hour() {
		day = now.Day() + 1
	} else {
		day = now.Day()
	}
	activeEndTime = time.Date(now.Year(), now.Month(), day, int(activeHoursEnd), 0, 0, 0, now.Location())

	return activeStartTime, activeEndTime, nil
}
