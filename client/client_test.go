// Copyright 2020 Google LLC
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

package client

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLocalServiceServerRunning(t *testing.T) {
	tests := []struct {
		in  func(http.ResponseWriter, *http.Request)
		out bool
	}{{
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("forbidden"))
		}, false}, {
		func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "missing", 404)
		}, false}, {
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/status" {
				fmt.Fprintln(w, "ok")
			} else {
				http.Error(w, "missing", 404)
			}
		}, true},
	}
	for i, tt := range tests {
		ts := httptest.NewServer(http.HandlerFunc(tt.in))
		s := Test(ts.URL)
		if s != tt.out {
			t.Errorf("Test(%v) = %v, want %v", i, s, tt.out)
		}
	}
}

func TestLocalServiceServerStopped(t *testing.T) {
	tests := []struct {
		in  func(http.ResponseWriter, *http.Request)
		out bool
	}{{
		func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/status" {
				fmt.Fprintln(w, "Hello, client")
			}
		}, false},
	}
	for _, tt := range tests {
		ts := httptest.NewUnstartedServer(http.HandlerFunc(tt.in))
		s := Test(ts.URL)
		if s != tt.out {
			t.Errorf("Test(%v) = %v, want %v", ts.URL, s, tt.out)
		}
	}
}

func TestMakeURL(t *testing.T) {
	tests := []struct {
		inNames []string
		inPort  int
		out     []string
	}{
		{[]string{"a", "b", "c"}, 1,
			[]string{
				"http://localhost:1/schedule/a",
				"http://localhost:1/schedule/b",
				"http://localhost:1/schedule/c",
			}},
		{[]string{}, 80, []string{"http://localhost:80/schedule"}},
	}
	for _, tt := range tests {
		res := makeURL(tt.inPort, tt.inNames)
		if !cmp.Equal(res, tt.out) {
			t.Errorf("makeURL(%d, %v) returned diff (-want +got): %v",
				tt.inPort, tt.inNames, cmp.Diff(res, tt.out))
		}
	}
}
