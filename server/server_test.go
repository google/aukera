// Copyright 2023 Google LLC
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

package server

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/google/aukera/window"
)

func TestHandler(t *testing.T) {
	tests := []struct {
		desc     string
		inURL    string
		fn       func(...string) ([]window.Schedule, error)
		wantCode int
		wantErr  error
	}{
		{
			desc:     "/status success",
			wantCode: 200,
			inURL:    "/status",
		},
		{
			desc:     "base schedule with error",
			wantCode: 500,
			inURL:    "/schedule",
			fn: func(names ...string) ([]window.Schedule, error) {
				return nil, errors.New("schedule error")
			},
		},
		{
			desc:     "schedule label with success",
			wantCode: 200,
			inURL:    "/schedule/specific",
			fn: func(names ...string) ([]window.Schedule, error) {
				if len(names) != 1 {
					t.Errorf("expected 1 argment, got: %d", len(names))
				}
				if names[0] != "specific" {
					t.Errorf("schedule called with unexpected argument: want specific, got %q", names[0])
				}
				return nil, nil
			},
		},
		{
			desc:     "schedule label with error",
			wantCode: 500,
			inURL:    "/schedule/specific",
			fn: func(names ...string) ([]window.Schedule, error) {
				return nil, errors.New("schedule error")
			},
		},
		{
			desc:     "invalid path",
			wantCode: 404,
			inURL:    "/missing",
			fn: func(names ...string) ([]window.Schedule, error) {
				return nil, nil
			},
		},
	}
	for _, tt := range tests {
		fnSchedule = tt.fn
		srv := httptest.NewServer(muxRouter())
		defer srv.Close()

		client := srv.Client()
		url := srv.URL + tt.inURL
		res, err := client.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		if err != tt.wantErr {
			t.Errorf("%s: produced unexpected error %v", tt.desc, err)
		}
		if res.StatusCode != tt.wantCode {
			t.Errorf("%s: produced unexpected status code: got %d, want %d", tt.desc, res.StatusCode, tt.wantCode)
		}
	}
}
