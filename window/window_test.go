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

package window

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp"
	"github.com/google/go-cmp/cmpopts"
	"github.com/robfig/cron/v3"
	"github.com/google/logger"
)

func testData(now time.Time) ([]Window, error) {
	var testData = []struct {
		JSON            []byte
		Starts, Expires time.Time
	}{
		{
			JSON: []byte(
				`{
			"Windows":
				[
					{
						"Name": "started not expired",
						"Format": 1,
						"Schedule": "* 0 */1 * * *",
						"Duration": "1h",
						"Labels": ["calculateSchedule"]
					}
				]
			}`,
			),
			Starts:  now.Add(-24 * time.Hour),
			Expires: now.Add(24 * time.Hour),
		},
		{
			JSON: []byte(
				`{
			"Windows":
				[
					{
						"Name": "not started",
						"Format": 1,
						"Schedule": "* 0 */1 * * *",
						"Duration": "1h",
						"Labels": ["calculateSchedule"]
					}
				]
			}`,
			),
			Starts:  now.Add(1 * time.Hour),
			Expires: now.Add(24 * time.Hour),
		},
		{
			JSON: []byte(
				`{
			"Windows":
				[
					{
						"Name": "expired",
						"Format": 1,
						"Schedule": "* 0 */1 * * *",
						"Duration": "1h",
						"Labels": ["calculateSchedule"]
					}
				]
			}`,
			),
			Expires: now.Add(-1 * time.Hour),
		},
		{
			JSON: []byte(
				`{
			"Windows":
				[
					{
						"Name": "started no expiry",
						"Format": 1,
						"Schedule": "* 0 */1 * * *",
						"Duration": "1h",
						"Labels": ["calculateSchedule"]
					}
				]
			}`,
			),
			Starts: now.Add(-1 * time.Hour),
		},
	}
	var windows []Window
	for _, d := range testData {
		var s = struct {
			Windows []Window
		}{}
		if err := json.Unmarshal(d.JSON, &s); err != nil {
			return nil, err
		}
		for _, w := range s.Windows {
			if !d.Starts.IsZero() {
				w.Starts = d.Starts
			}
			if !d.Expires.IsZero() {
				w.Expires = d.Expires
			}
			w.calculateSchedule()
			windows = append(windows, w)
		}
	}

	return windows, nil
}

func labels(windows []Window) (out []string) {
	contains := func(sl []string, s string) bool {
		for i := range sl {
			if sl[i] == s {
				return true
			}
		}
		return false
	}
	for _, w := range windows {
		for _, l := range w.Labels {
			if !contains(out, l) {
				out = append(out, l)
			}
		}
	}
	return out
}

func TestUnmarshalWindow(t *testing.T) {
	var testWindowJSON = []struct {
		desc        string
		json        []byte
		expectError bool
	}{
		{
			"full window config",
			[]byte(
				`{
		"Windows":
			[
				{
					"Name": "always open",
					"Format": 1,
					"Schedule": "* * * * * *",
					"Duration": "2m",
					"Starts": "2019-01-01T23:00:00Z",
					"Expires": "2020-01-01T23:00:00Z",
					"Labels": ["default"]
				}
			]
		}`),
			false,
		},
		{
			"minimum window config",
			[]byte(
				`{
		"Windows":
			[
				{
					"Name": "minimum",
					"Format": 1,
					"Schedule": "* * * * * *",
					"Duration": "2m",
					"Labels": ["default"]
				}
			]
		}`),
			false,
		},
		{
			"invalid format type",
			[]byte(
				`{
		"Windows":
			[
				{
					"Name": "invalid format type",
					"Format": 2,
					"Schedule": "* * * * * *",
					"Duration": "2m",
					"Labels": ["default"]
				}
			]
		}`),
			true,
		},
		{
			"no label",
			[]byte(
				`{
		"Windows":
			[
				{
					"Name": "no label",
					"Format": 1,
					"Schedule": "* * * * * *",
					"Duration": "2m"
				}
			]
		}`),
			true,
		},
		{
			"empty name",
			[]byte(
				`{
		"Windows":
			[
				{
					"Name": "",
					"Format": 1,
					"Schedule": "* * * * * *",
					"Duration": "2m"
					"Label": ["default"]
				}
			]
		}`),
			true,
		},
		{
			"no name field",
			[]byte(
				`{
		"Windows":
			[
				{
					"Format": 1,
					"Schedule": "* * * * * *",
					"Duration": "2m"
					"Label": ["default"]
				}
			]
		}`),
			true,
		},
		{"nil json",
			nil,
			true,
		},
		{"invalid json",
			[]byte(`{["Window" : true]`),
			true,
		},
	}
	for _, j := range testWindowJSON {
		s := struct {
			Windows []Window
		}{}
		if err := json.Unmarshal(j.json, &s); (err != nil) != j.expectError {
			t.Errorf("TestUnmarshalWindow(%q) errors occurred: %t; expected: %t (error: %v)", j.desc, (err != nil), j.expectError, err)
		}
	}
}

func TestCalculateSchedule(t *testing.T) {
	var (
		m         = make(Map)
		now       = time.Now()
		dur       = 1 * time.Hour
		testLabel = "calculateSchedule"
		tests     = []struct {
			windowName string
			expect     Schedule
		}{
			{"started not expired",
				Schedule{
					State:    "open",
					Duration: dur,
					Opens:    now.Truncate(time.Hour),
					Closes:   now.Truncate(time.Hour).Add(dur),
				},
			},
			{"not started",
				Schedule{
					State:    "closed",
					Duration: dur,
					Opens:    now.Truncate(time.Hour).Add(2 * time.Hour),
					Closes:   now.Truncate(time.Hour).Add((2 * time.Hour) + dur),
				},
			},
			{"expired",
				Schedule{
					State:    "closed",
					Duration: dur,
					Opens:    now.Truncate(time.Hour).Add(-2 * time.Hour),
					Closes:   now.Truncate(time.Hour).Add(-1 * time.Hour),
				},
			},
			{"started no expiry",
				Schedule{
					State:    "open",
					Duration: dur,
					Opens:    now.Truncate(time.Hour),
					Closes:   now.Truncate(time.Hour).Add(1 * time.Hour),
				},
			},
		}
	)
	// Populate Window Map
	d, err := testData(time.Now())
	if err != nil {
		t.Fatalf("TestCalculateSchedule(): error getting test data: %v", err)
	}
	m.Add(d...)

	for _, e := range tests {
		w := m.FindWindow(e.windowName, testLabel)
		got := w.Schedule
		if got.State != e.expect.State {
			t.Errorf("TestCalculateSchedule(%q) state:: got: %s; want: %s", e.windowName, got.State, e.expect.State)
		}
		if got.Duration != e.expect.Duration {
			var (
				gotDur    = got.Duration.String()
				expectDur = e.expect.Duration.String()
			)
			t.Errorf("TestCalculateSchedule(%q) duration:: got: %s; want: %s", e.windowName, gotDur, expectDur)
		}
		if got.Opens != e.expect.Opens {
			t.Errorf("TestCalculateSchedule(%q) opens:: got: %s; want: %s", e.windowName, got.Opens, e.expect.Opens)
		}
		if got.Closes != e.expect.Closes {
			t.Errorf("TestCalculateSchedule(%q) closes:: got: %s; want: %s", e.windowName, got.Closes, e.expect.Closes)
		}
	}
}

func TestWindowMarshal(t *testing.T) {
	tests, err := testData(time.Now())
	if err != nil {
		t.Fatalf("TestWindowMarshal(): error getting test data: %v", err)
	}
	for _, w := range tests {
		if _, err := json.Marshal(w); err != nil {
			t.Fatalf("TestWindowMarshal(%q): unexpected error marshaling Window: %v", w.Name, err)
		}
	}
}

func TestMapKeys(t *testing.T) {
	tests, err := testData(time.Now())
	if err != nil {
		t.Fatalf("TestWindowMarshal(): error getting test data: %v", err)
	}

	m := make(Map)
	m.Add(tests...)

	tfrm := cmp.Transformer("Sort", func(in []string) []string {
		out := append([]string(nil), in...) // Copy input to avoid mutating it
		sort.Strings(out)
		return out
	})
	if !cmp.Equal(m.Keys(), labels(tests), tfrm) {
		t.Errorf("TestMapKeys(): keys don't match: got: %s; want: %s", m.Keys(), labels(tests))
	}
}

func TestMapFind(t *testing.T) {
	tests, err := testData(time.Now())
	if err != nil {
		t.Fatalf("TestWindowMarshal(): error getting test data: %v", err)
	}

	m := make(Map)
	m.Add(tests...)

	for _, l := range labels(tests) {
		if w := m.Find(l); len(w) == 0 {
			t.Errorf("TestMapFind(%q): failed to find windows that match label.", l)
		}
	}
}

func TestMapMarshal(t *testing.T) {
	tests, err := testData(time.Now())
	if err != nil {
		t.Fatalf("TestWindowMarshal(): error getting test data: %v", err)
	}

	m := make(Map)
	m.Add(tests...)
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("TestMapMarshal(): unexpected error marshaling Map. got: %v; want: nil", err)
	}
	var nullJSON = []byte(`{"Windows":null}`)
	if cmp.Equal(b, nullJSON) {
		t.Errorf("TestMapMarshal(): received null JSON: got: %s", b)
	}
}

// ConfigReader Tests
func TestConfigReaderAbsPath(t *testing.T) {
	var r Reader
	pwd, err := os.Getwd()
	if err != nil {
		t.Errorf("TestReaderAbsPath(): failed to get working directory")
	}

	tests := []struct {
		desc, path, pathExpect string
		expectErr              bool
		errMsg                 string
	}{
		{
			desc:       "working directory",
			path:       "./",
			pathExpect: pwd,
			expectErr:  false,
		},
		{
			desc:       "non-existent path",
			path:       "made/this/rel/path/up",
			pathExpect: "",
			expectErr:  true,
			errMsg:     fmt.Sprintf("AbsPath: doesn't exist: %q", filepath.Join(pwd, "made/this/rel/path/up")),
		},
	}

	if runtime.GOOS == "Windows" {
		tests = append(tests, struct {
			desc, path, pathExpect string
			expectErr              bool
			errMsg                 string
		}{
			desc:       "windows invalid path",
			path:       `\*_*invalid+path,|.`,
			pathExpect: "",
			expectErr:  true,
			errMsg:     fmt.Sprintf("CreateFile %s: Invalid name.", filepath.Join(pwd, `\*_*invalid+path,|.`)),
		})
	}

	for _, test := range tests {
		p, err := r.AbsPath(test.path)
		if err != nil && !test.expectErr {
			t.Errorf("TestReaderAbsPath(%q): unexpected error: %v", test.desc, err)
		}
		if err != nil && test.expectErr {
			if err.Error() != test.errMsg {
				t.Errorf("TestReaderAbsPath(%q): unexpected error message: got: %v; want: %s", test.desc, err, test.errMsg)
			}
		}

		if p != test.pathExpect {
			t.Errorf("TestReaderAbsPath(%q): unexpected path returned: got: %s; want: %s", test.desc, p, test.pathExpect)
		}
	}
}

func TestWindowsPathNotExist(t *testing.T) {
	var (
		r    Reader
		test = struct {
			desc, path string
			expectErr  bool
		}{"non-existent path", "made/this/path/up", true}
	)

	pwd, err := os.Getwd()
	if err != nil {
		t.Errorf("TestWindowsPathNotExist(%q): failed to get working directory", test.desc)
	}
	m, err := Windows(test.path, r)
	if m != nil {
		t.Errorf("TestWindowsPathNotExist(%q): Map:: got: %+v; want: nil", test.desc, m)
	}
	if err == nil {
		errMsg := fmt.Sprintf("open %s: no such file or directory", filepath.Join(pwd, test.path))
		t.Errorf("TestWindowsPathNotExist(%q): error:: got: %v; want: %s", test.desc, err, errMsg)
	}
}

// mockFileInfo is used to abstract filesystem actions.
type mockFileInfo struct {
	os.FileInfo
	name string
}

func (mfi mockFileInfo) Name() string {
	return mfi.name
}

// Mock ConfigReader for window.Windows() tests
type TestReader struct {
	windows []Window
}

func (r TestReader) PathExists(path string) (bool, error) {
	return true, nil
}

func (r TestReader) AbsPath(path string) (string, error) {
	return path, nil
}

func (r TestReader) JSONFiles(path string) ([]os.FileInfo, error) {
	return []os.FileInfo{mockFileInfo{name: path}}, nil
}

func (r TestReader) JSONContent(path string) ([]byte, error) {
	if strings.ToLower(filepath.Ext(path)) != ".json" {
		return nil, fmt.Errorf("file is not JSON")
	}

	m := make(Map)
	m.Add(r.windows...)
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func TestWindows(t *testing.T) {
	windows, err := testData(time.Now().Local())
	if err != nil {
		t.Fatalf("TestWindows(): error getting test data: %v", err)
	}
	m := make(Map)
	m.Add(windows...)
	tests := []struct {
		desc, path, errRegex string
		mapExpect            Map
		expectErr            bool
	}{
		{
			desc:      "no json",
			path:      "conf/notjson.yml",
			mapExpect: Map{},
			expectErr: true,
			errRegex:  `.*? error reading file \"conf/notjson.yml\": file is not JSON\s?`,
		},
		{
			desc:      "use testData",
			path:      "conf/config.json",
			mapExpect: m,
			expectErr: false,
		},
	}

	r := TestReader{windows}
	var logBuffer bytes.Buffer
	logger.Init("TestWindows", false, false, &logBuffer)

	for _, tst := range tests {
		m, _ := Windows(tst.path, r)

		if tst.expectErr {
			errMsg := logBuffer.String()
			errMatch, err := regexp.MatchString(tst.errRegex, errMsg)
			if err != nil {
				t.Errorf("TestWindows(%q): error occurred parsing test regex %q: %v", tst.desc, tst.errRegex, err)
			}
			if !errMatch {
				t.Errorf("TestWindows(%q): unexpected error message: %q did not match regex %q", tst.desc, errMsg, tst.errRegex)
			}
		}
		if diff := cmp.Diff(m, tst.mapExpect, cmpopts.IgnoreFields(cron.SpecSchedule{}, "Location")); diff != "" {
			t.Errorf("TestWindows(%q): produced unexpected diff: %s", tst.desc, diff)
		}
		logBuffer.Reset()
	}
}

func TestWindowActivation(t *testing.T) {
	src := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.Local)
	activationTests := []struct {
		desc, cron       string
		time, next, last time.Time
	}{
		{"every minute", "* * * * * *", src.Add(10 * time.Second), src, src.Add(-1 * time.Minute)},
		{"every 2 minutes [even start]", "* */2 * * * *", src.Add(10 * time.Second), src, src.Add(-2 * time.Minute)},
		{"every 2 minutes [odd start]", "* */2 * * * *", src.Add(1 * time.Minute), src.Add(2 * time.Minute), src},
		{"next month", "* * * * 2 *", src, src.AddDate(0, 1, 0), src.AddDate(-1, 1, 0)},
		{"next year", "* 0 0 1 1 *", src.Add(1 * time.Hour), src.AddDate(1, 0, 0), src},
	}
	for _, a := range activationTests {
		// Default parser removed in cron v3; manually specifying default cron parser.
		p := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.DowOptional | cron.Descriptor)
		cr, err := p.Parse(a.cron)
		if err != nil {
			t.Errorf("TestActivation(%q) error parsing cron string %q: %v", a.desc, a.cron, err)
		}

		w := Window{Format: 1, Cron: cr}
		last := w.LastActivation(a.time)
		if last != a.last {
			t.Errorf("TestActivation(%q) last activation: got: %s; want: %s", a.desc, last, a.last)
		}

		next := w.NextActivation(a.time)
		if next.IsZero() {
			t.Errorf("TestActivation(%q) next activation search timeout exceeded.", a.desc)
		}

		if next != a.next {
			t.Errorf("TestActivation(%q) next activation: got: %s; want: %s", a.desc, next, a.next)
		}
	}
}

type schedules struct {
	schedA       Schedule
	schedOverlap Schedule
	schedB       Schedule
	schedBig     Schedule
}

type compTest struct {
	desc                    string
	base, compare, combined Schedule
	overlaps                bool
}

func (s *schedules) comparisonTests() []compTest {
	return []compTest{
		{"base in compare",
			s.schedA,
			s.schedBig,
			s.schedBig,
			true,
		},
		{"base later than compare",
			s.schedOverlap,
			s.schedA,
			Schedule{
				Opens:    s.schedA.Opens,
				Closes:   s.schedOverlap.Closes,
				Duration: (7 * time.Minute),
			},
			true,
		},
		{"base earlier than compare",
			s.schedA,
			s.schedOverlap,
			Schedule{
				Opens:    s.schedA.Opens,
				Closes:   s.schedOverlap.Closes,
				Duration: (7 * time.Minute),
			},
			true,
		},
		{"base matches compare",
			s.schedA,
			s.schedA,
			s.schedA,
			true,
		},
		{"no overlap",
			s.schedA,
			s.schedB,
			s.schedA,
			false,
		},
	}
}

// templated schedules
func makeSchedules(now time.Time) schedules {
	return schedules{
		schedA: Schedule{
			Opens:    now.Add(-5 * time.Minute),
			Closes:   now,
			Duration: (5 * time.Minute),
		},
		schedOverlap: Schedule{
			Opens:    now.Add(-2 * time.Minute),
			Closes:   now.Add(2 * time.Minute),
			Duration: (4 * time.Minute),
		},
		schedB: Schedule{
			Opens:    now,
			Closes:   now.Add(5 * time.Minute),
			Duration: (5 * time.Minute),
		},
		schedBig: Schedule{
			Opens:    now.Add(-5 * time.Minute),
			Closes:   now.Add(10 * time.Minute),
			Duration: (15 * time.Minute),
		},
	}
}

func TestScheduleOverlaps(t *testing.T) {
	s := makeSchedules(time.Now().Local())
	for _, e := range s.comparisonTests() {
		if overlaps := e.base.Overlaps(e.compare); overlaps != e.overlaps {
			t.Errorf("TestScheduleOverlaps(%q) got %t; want %t", e.desc, e.overlaps, overlaps)
		}
	}
}

func TestScheduleCombine(t *testing.T) {
	s := makeSchedules(time.Now().Local())
	for _, e := range s.comparisonTests() {
		err := e.base.Combine(e.compare)
		if err != nil && e.overlaps {
			t.Errorf("TestScheduleCombine(%q) error: %v", e.desc, err)
		}
		if e.base.Opens != e.combined.Opens {
			t.Errorf("TestScheduleCombine(%q) incorrect opening time. got: %s; want: %s", e.desc, e.base.Opens, e.combined.Opens)
		}
		if e.base.Closes != e.combined.Closes {
			t.Errorf("TestScheduleCombine(%q) incorrect closing time. got: %s; want: %s", e.desc, e.base.Closes, e.combined.Closes)
		}
		dur := e.combined.Closes.Sub(e.combined.Opens)
		if e.base.Duration != dur {
			t.Errorf("TestScheduleCombine(%q) incorrect duration. got: %s; want %s", e.desc, e.base.Duration.String(), dur.String())
		}
	}
}

func TestScheduleOpen(t *testing.T) {
	dur, err := time.ParseDuration("20m")
	if err != nil {
		t.Errorf("error parsing duration: %v", err)
	}
	open := Schedule{
		State:    "open",
		Opens:    time.Now().Add(-10 * time.Minute),
		Closes:   time.Now().Add(10 * time.Minute),
		Duration: dur,
	}

	if !open.IsOpen() {
		t.Errorf("open schedule (%s for %s) indicates closed status", open.Opens, dur.String())
	}
}

func TestScheduleClosed(t *testing.T) {
	dur, err := time.ParseDuration("20m")
	if err != nil {
		t.Errorf("error parsing duration: %v", err)
	}
	open := Schedule{
		State:    "closed",
		Opens:    time.Now().Add(10 * time.Minute),
		Closes:   time.Now().Add(20 * time.Minute),
		Duration: dur,
	}

	if open.IsOpen() {
		t.Errorf("closed schedule (%s for %s) indicates open status", open.Opens, dur.String())
	}
}

func TestDedupSchedules(t *testing.T) {
	s := makeSchedules(time.Now().Local())
	test := struct {
		input, want []Schedule
	}{
		input: []Schedule{s.schedA, s.schedA, s.schedB, s.schedOverlap, s.schedB, s.schedBig},
		want:  []Schedule{s.schedA, s.schedB, s.schedOverlap, s.schedBig},
	}
	sort.Slice(test.want, func(i int, j int) bool {
		return test.want[i].Opens.Before(test.want[j].Opens)
	})
	unique := dedupSchedules(test.input)
	sort.Slice(unique, func(i int, j int) bool {
		return unique[i].Opens.Before(unique[j].Opens)
	})
	if !cmp.Equal(unique, test.want) {
		t.Errorf("TestDedupSchedules(): got: %v; want: %v", unique, test.want)
	}
}

func TestScheduleMarshal(t *testing.T) {
	d, err := time.ParseDuration("1h0m0s")
	if err != nil {
		t.Fatalf("TestScheduleMarshal(): unable to parse test duration: %v", err)
	}
	open := time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local)
	closed := time.Date(2020, 1, 1, 1, 0, 0, 0, time.Local)
	test := struct {
		desc      string
		sched     Schedule
		want      []byte
		expectErr bool
	}{
		"should marshal",
		Schedule{
			Name:     "should marshal",
			State:    "closed",
			Duration: d,
			Opens:    open,
			Closes:   closed,
		},
		[]byte(fmt.Sprintf(`{"Name":"should marshal","State":"closed","Opens":%q,"Closes":%q,"Duration":"1h0m0s"}`, open.Format(time.RFC3339), closed.Format(time.RFC3339))),
		false,
	}

	b, err := json.Marshal(&test.sched)
	if (err != nil) != test.expectErr {
		t.Errorf("TestScheduleMarshal(%q): unexpected error: %v", test.desc, err)
	}
	if !cmp.Equal(b, test.want) {
		t.Errorf("TestScheduleMarshal(%q): unexpected JSON returned: got: %s; want: %s", test.desc, string(b), string(test.want))
	}
}
