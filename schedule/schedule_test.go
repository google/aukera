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

package schedule

import (
	"testing"
	"time"

	"github.com/google/aukera/window"
)

type ts map[string]window.Schedule

func (t ts) vals() []window.Schedule {
	var out []window.Schedule
	for _, v := range t {
		out = append(out, v)
	}
	return out
}

var (
	now           = time.Now()
	testSchedules = ts{
		"plus_10_days": window.Schedule{
			Name:   "plus_10_days",
			Opens:  now.Add(10 * (24 * time.Hour)),
			Closes: now.Add(10*(24*time.Hour) + (6 * time.Hour)),
		},
		"minus_14_days": window.Schedule{
			Name:   "minus_14_days",
			Opens:  now.Add(-(14 * (24 * time.Hour))),
			Closes: now.Add(-(14*(24*time.Hour) + (8 * time.Hour))),
		},
		"minus_6_days": window.Schedule{
			Name:   "minus_6_days",
			Opens:  now.Add(-(6 * (24 * time.Hour))),
			Closes: now.Add(-(6*(24*time.Hour) + (6 * time.Hour))),
		},
		"plus_2_days": window.Schedule{
			Name:   "plus_2_days",
			Opens:  now.Add(2 * (24 * time.Hour)),
			Closes: now.Add(2*(24*time.Hour) + (4 * time.Hour)),
		},
		"plus_30_days": window.Schedule{
			Name:   "plus_30_days",
			Opens:  now.Add(30 * (24 * time.Hour)),
			Closes: now.Add(30*(24*time.Hour) + (4 * time.Hour)),
		},
	}
)

func modSched(add ts, del []string) map[string]window.Schedule {
	s := make(ts)
	for v := range testSchedules {
		s[v] = testSchedules[v]
	}
	for v := range add {
		s[v] = add[v]
	}
	for _, v := range del {
		delete(s, v)
	}
	return s
}

func TestFindNearest(t *testing.T) {
	tests := []struct {
		in   ts
		want string
	}{
		// simple future schedule
		{testSchedules, "plus_2_days"},
		// active open window
		{modSched(ts{
			"test_open_now": window.Schedule{
				Name:   "test_open_now",
				Opens:  now.Add(-(2 * time.Hour)),
				Closes: now.Add(2 * time.Hour),
			}}, nil), "test_open_now"},
		// all in past
		{modSched(nil,
			[]string{"plus_2_days", "plus_10_days", "plus_30_days"}), "minus_6_days"},
	}
	for _, tt := range tests {
		res := findNearest(tt.in.vals())
		if res != tt.in[tt.want] {
			t.Errorf("findNearest(%v) = %v, want (%v)", tt.in, res, tt.in[tt.want])
		}
	}
}
