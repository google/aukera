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
	"io/ioutil"
	"net/http"

	"../window"
)

const (
	urlBase = "http://localhost"
)

// Test validates service is available and responding locally.
func Test(port int) bool {
	response, err := http.Get(fmt.Sprintf("%s:%d/status", urlBase, port))
	if err != nil {
		return false
	}
	return response.StatusCode == http.StatusOK
}

// Label gets a window schedule by label name(s).
func Label(port int, names ...string) ([]window.Schedule, error) {
	if !Test(port) {
		return nil, fmt.Errorf("service not available")
	}
	var urls []string
	if len(names) == 0 {
		urls = append(urls, fmt.Sprintf("%s:%d/schedule", urlBase, port))
	} else {
		for _, name := range names {
			urls = append(urls, fmt.Sprintf("%s:%d/schedule/%s", urlBase, port, name))
		}
	}
	var sched []window.Schedule
	for _, url := range urls {
		response, err := http.Get(url)
		if err != nil {
			return nil, err
		}
		j, err := ioutil.ReadAll(response.Body)
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
