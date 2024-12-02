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

// Package client provides a library for other services to query Aukera for information.
package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/aukera/window"
)

const (
	urlBase = "http://localhost"
)

// Test validates service is available and responding locally.
func Test(url string) bool {
	response, err := http.Get(fmt.Sprintf("%s/status", url))
	if err != nil {
		return false
	}
	defer response.Body.Close()

	return response.StatusCode == http.StatusOK
}

func makeURL(port int, names []string) []string {
	var urls []string
	if len(names) == 0 {
		urls = append(urls, fmt.Sprintf("%s:%d/schedule", urlBase, port))
	} else {
		for _, name := range names {
			urls = append(urls, fmt.Sprintf("%s:%d/schedule/%s", urlBase, port, name))
		}
	}
	return urls
}

// Label gets a window schedule by label name(s).
func Label(port int, names ...string) ([]window.Schedule, error) {
	if !Test(fmt.Sprintf("%s:%d", urlBase, port)) {
		return nil, fmt.Errorf("service not available")
	}
	urls := makeURL(port, names)
	return readSchedules(urls)
}

// ActiveHours gets the built-in Active Hours maintenance window.
// This window is active (open) during the times set by the user or machine admin as the hours
// during which user activity is expected.
func ActiveHours(port int) (*window.Window, error) {
	if !Test(fmt.Sprintf("%s:%d", urlBase, port)) {
		return nil, fmt.Errorf("service not available")
	}
	url := fmt.Sprintf("%s:%d/active_hours", urlBase, port)
	var win *window.Window
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return win, fmt.Errorf(
			"active_hours request failed for url %s (%d)", url, response.StatusCode)
	}
	j, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(j, win); err != nil {
		return nil, err
	}
	return win, nil
}

func readSchedules(urls []string) ([]window.Schedule, error) {
	var sched []window.Schedule
	for _, url := range urls {
		response, err := http.Get(url)
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return sched, fmt.Errorf(
				"schedule request failed for url %s (%d)", url, response.StatusCode)
		}
		j, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}

		var s []window.Schedule
		if err := json.Unmarshal(j, &s); err != nil {
			return nil, err
		}
		sched = append(sched, s...)
	}
	return sched, nil
}
