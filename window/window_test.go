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
	_ "time/tzdata"

	"github.com/google/deck/backends/logger"
	"github.com/google/deck"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/robfig/cron/v3"
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
		if !got.Opens.Equal(e.expect.Opens) {
			t.Errorf("TestCalculateSchedule(%q) opens:: got: %s; want: %s", e.windowName, got.Opens, e.expect.Opens)
		}
		if !got.Closes.Equal(e.expect.Closes) {
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

// mockDirEntry is used to abstract filesystem actions.
type mockDirEntry struct {
	os.DirEntry
	name string
}

func (mfi mockDirEntry) Name() string {
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

func (r TestReader) JSONFiles(path string) ([]os.DirEntry, error) {
	return []os.DirEntry{mockDirEntry{name: path}}, nil
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
	deck.Add(logger.Init(&logBuffer, 0))

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
		if !last.Equal(a.last) {
			t.Errorf("TestActivation(%q) last activation: got: %s; want: %s", a.desc, last, a.last)
		}

		next := w.NextActivation(a.time)
		if next.IsZero() {
			t.Errorf("TestActivation(%q) next activation search timeout exceeded.", a.desc)
		}

		if !next.Equal(a.next) {
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
		if !e.base.Opens.Equal(e.combined.Opens) {
			t.Errorf("TestScheduleCombine(%q) incorrect opening time. got: %s; want: %s", e.desc, e.base.Opens, e.combined.Opens)
		}
		if !e.base.Closes.Equal(e.combined.Closes) {
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

func TestNthWeekdaySchedule(t *testing.T) {
	locNY, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("LoadLocation failed: %v", err)
	}

	apr1NY := time.Date(2025, time.April, 1, 0, 0, 0, 0, locNY)
	tue3rdNY := time.Date(2025, time.April, 15, 17, 0, 0, 0, locNY)
	tue3rdMarNY := time.Date(2025, time.March, 18, 17, 0, 0, 0, locNY)

	apr1Local := time.Date(2025, time.April, 1, 0, 0, 0, 0, time.Local)
	tue3rdLocal := time.Date(2025, time.April, 15, 17, 0, 0, 0, time.Local)
	tue3rdMarLocal := time.Date(2025, time.March, 18, 17, 0, 0, 0, time.Local)

	apr1UTC := time.Date(2025, time.April, 1, 0, 0, 0, 0, time.UTC)
	tue3rdUTC := time.Date(2025, time.April, 15, 17, 0, 0, 0, time.UTC)
	tue3rdMarUTC := time.Date(2025, time.March, 18, 17, 0, 0, 0, time.UTC)

	validTests := []struct {
		desc               string
		spec               string
		from               time.Time
		wantNext, wantLast time.Time
	}{
		{"3rd Tue TZ=America/New_York (5 fields)", "TZ=America/New_York 0 17 * * Tue#3", apr1NY, tue3rdNY, tue3rdMarNY},
		{"3rd Tue TZ=America/New_York (6 fields)", "TZ=America/New_York 0 0 17 * * Tue#3", apr1NY, tue3rdNY, tue3rdMarNY},
		{"3rd Tue CRON_TZ=America/New_York (5 fields)", "CRON_TZ=America/New_York 0 17 * * Tue#3", apr1NY, tue3rdNY, tue3rdMarNY},
		{"3rd Tue CRON_TZ=America/New_York (6 fields)", "CRON_TZ=America/New_York 0 0 17 * * Tue#3", apr1NY, tue3rdNY, tue3rdMarNY},
		{"3rd Tue Tue#3 (6 fields)", "0 0 17 * * Tue#3", apr1Local, tue3rdLocal, tue3rdMarLocal},
		{"3rd Tue numeric 2#3", "0 0 17 * * 2#3", apr1Local, tue3rdLocal, tue3rdMarLocal},
		{"3rd Tue 5-field cron", "0 17 * * Tue#3", apr1Local, tue3rdLocal, tue3rdMarLocal},
		{"3rd Tue after occurrence passed", "0 0 17 * * Tue#3", time.Date(2025, time.April, 16, 0, 0, 0, 0, time.Local), time.Date(2025, time.May, 20, 17, 0, 0, 0, time.Local), tue3rdLocal},
		{"Last Tue Tue#L", "0 0 17 * * Tue#L", apr1Local, time.Date(2025, time.April, 29, 17, 0, 0, 0, time.Local), time.Date(2025, time.March, 25, 17, 0, 0, 0, time.Local)},
		{"1st Mon Mon#1", "0 0 9 * * Mon#1", apr1Local, time.Date(2025, time.April, 7, 9, 0, 0, 0, time.Local), time.Date(2025, time.March, 3, 9, 0, 0, 0, time.Local)},
		{"3rd Tue CRON_TZ=UTC", "CRON_TZ=UTC 0 0 17 * * Tue#3", apr1UTC, tue3rdUTC, tue3rdMarUTC},
		{"3rd Tue CRON_TZ=UTC 5-field", "CRON_TZ=UTC 0 17 * * Tue#3", apr1UTC, tue3rdUTC, tue3rdMarUTC},
		{"Last Tue Tue#LAST", "0 0 17 * * Tue#LAST", apr1Local, time.Date(2025, time.April, 29, 17, 0, 0, 0, time.Local), time.Date(2025, time.March, 25, 17, 0, 0, 0, time.Local)},
		{"5th Tue month with 4 skips", "0 0 17 * * Tue#5", time.Date(2025, time.May, 1, 0, 0, 0, 0, time.Local), time.Date(2025, time.July, 29, 17, 0, 0, 0, time.Local), time.Date(2025, time.April, 29, 17, 0, 0, 0, time.Local)},
		{"1st Sun 0#1", "0 0 10 * * 0#1", apr1Local, time.Date(2025, time.April, 6, 10, 0, 0, 0, time.Local), time.Date(2025, time.March, 2, 10, 0, 0, 0, time.Local)},
		{"1st Sun 7#1", "0 0 10 * * 7#1", apr1Local, time.Date(2025, time.April, 6, 10, 0, 0, 0, time.Local), time.Date(2025, time.March, 2, 10, 0, 0, 0, time.Local)},
		{"3rd Tue whitespace CRON_TZ=UTC", "   CRON_TZ=UTC 0 0 17 * * Tue#3   ", apr1UTC, tue3rdUTC, tue3rdMarUTC},
		{"3rd Tue whitespace 5-field CRON_TZ=UTC", "   CRON_TZ=UTC 0 17 * * Tue#3   ", apr1UTC, tue3rdUTC, tue3rdMarUTC},
		{"3rd Tue whitespace without TZ", "   0 0 17 * * Tue#3   ", apr1Local, tue3rdLocal, tue3rdMarLocal},
		{"3rd Tue whitespace 5-field without TZ", "   0 17 * * Tue#3   ", apr1Local, tue3rdLocal, tue3rdMarLocal},
	}
	for _, tc := range validTests {
		t.Run(tc.desc, func(t *testing.T) {
			sched, err := parseSchedule(tc.spec)
			if err != nil {
				t.Fatalf("parseSchedule(%q): unexpected error: %v", tc.spec, err)
			}
			w := Window{Format: FormatCron, Cron: sched, CronString: tc.spec}
			if got := w.NextActivation(tc.from); !got.Equal(tc.wantNext) {
				t.Errorf("NextActivation(%q): got %s, want %s", tc.spec, got, tc.wantNext)
			}
			if got := w.LastActivation(tc.from); !got.Equal(tc.wantLast) {
				t.Errorf("LastActivation(%q): got %s, want %s", tc.spec, got, tc.wantLast)
			}
		})
	}

	errorTests := []struct {
		desc, spec, wantErrSubstr string
	}{
		{"invalid occurrence #0", "0 0 17 * * Tue#0", "must be 1-5 or L"},
		{"invalid occurrence #6", "0 0 17 * * Tue#6", "must be 1-5 or L"},
		{"invalid weekday name", "0 0 17 * * Foo#3", "unknown weekday"},
		{"invalid weekday number 8", "0 0 17 * * 8#1", "unknown weekday"},
		{"hash in day-of-month field", "0 0 17 *#3 * Tue", "found in other field"},
		{"hash in minute field 6-field cron", "0 17#3 0 * * Tue", "found in other field"},
		{"hash in hour field 6-field cron", "0 0 17#3 * * Tue", "found in other field"},
		{"hash in month field 6-field cron", "0 0 17 * 4#3 Tue", "found in other field"},
		{"missing occurrence after hash", "0 0 17 * * Tue#", "must be 1-5 or L"},
		{"missing weekday before hash", "0 0 17 * * #3", "unknown weekday"},
		{"multiple hashes", "0 0 17 * * Tue#3#1", "invalid '#' syntax"},
		{"non-numeric occurrence", "0 0 17 * * Tue#foo", "must be 1-5 or L"},
		{"wrong field count with hash", "0 0 0 17 * * Tue#3", "expected 6 fields"},
		{"invalid base schedule hour", "0 0 99 * * Tue#3", "error parsing base schedule"},
		{"hash in day-of-month field 5-field cron", "17 *#3 * * Tue", "found in other field"},
		{"hash in minute field 5-field cron", "17#3 0 * * Tue", "found in other field"},
		{"hash in hour field 5-field cron", "0 17#3 * * Tue", "found in other field"},
		{"hash in month field 5-field cron", "0 17 * 4#3 Tue", "found in other field"},
		{"hash in month field with TZ 5-field cron", "TZ=America/New_York 0 17 * 4#3 Tue", "found in other field"},
		{"hash in month field with TZ 6-field cron", "TZ=America/New_York 0 0 17 * 4#3 Tue", "found in other field"},
		{"4 fields with hash", "17 * * Tue#3", "expected 6 fields"},
		{"only TZ with hash and no fields", "CRON_TZ=UTC#1", "expected 6 fields"},
		{"only TZ prefix with hash and no fields", "TZ=UTC#1", "expected 6 fields"},
	}
	for _, tc := range errorTests {
		t.Run(tc.desc, func(t *testing.T) {
			_, err := parseSchedule(tc.spec)
			if err == nil {
				t.Fatalf("parseSchedule(%q): expected error, got nil", tc.spec)
			}
			if !strings.Contains(err.Error(), tc.wantErrSubstr) {
				t.Errorf("parseSchedule(%q): error %q does not contain expected substring %q", tc.spec, err, tc.wantErrSubstr)
			}
		})
	}
}

func TestNthWeekdayWindowJSONAndUnique(t *testing.T) {
	rawJSON := []byte(`{"Windows": [
		{"Name": "patching-window", "Format": 1, "Schedule": "0 0 17 * * Tue#3", "Duration": "3h", "Labels": ["patching", "reboot"]},
		{"Name": "patching-window", "Format": 1, "Schedule": "0 0 17 * * Tue#3", "Duration": "3h", "Labels": ["patching", "reboot"]}
	]}`)

	var s struct{ Windows []Window }
	if err := json.Unmarshal(rawJSON, &s); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if len(s.Windows) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(s.Windows))
	}

	m := make(Map)
	m.Add(s.Windows...)
	unique := m.UniqueWindows()
	if len(unique) != 1 {
		t.Errorf("UniqueWindows(): expected 1 unique window, got %d", len(unique))
	}
	if diff := cmp.Diff(unique[0], s.Windows[0], cmpopts.IgnoreFields(cron.SpecSchedule{}, "Location")); diff != "" {
		t.Errorf("UniqueWindows(): unexpected difference (-got +want):\n%s", diff)
	}

	mBytes, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("json.Marshal(m) failed: %v", err)
	}
	var remarshaled struct{ Windows []Window }
	if err := json.Unmarshal(mBytes, &remarshaled); err != nil {
		t.Fatalf("json.Unmarshal(mBytes) failed: %v", err)
	}
	if len(remarshaled.Windows) != 1 {
		t.Errorf("expected 1 window after re-marshaling Map, got %d", len(remarshaled.Windows))
	} else if diff := cmp.Diff(remarshaled.Windows[0], unique[0], cmpopts.IgnoreFields(cron.SpecSchedule{}, "Location")); diff != "" {
		t.Errorf("remarshaled window diff (-got +want):\n%s", diff)
	}

	var distinct Window
	if err := json.Unmarshal([]byte(`{"Name": "maintenance-window", "Format": 1, "Schedule": "0 0 18 * * Wed#1", "Duration": "2h", "Labels": ["maintenance"]}`), &distinct); err != nil {
		t.Fatalf("json.Unmarshal(distinct) failed: %v", err)
	}
	m.Add(distinct)
	if got := len(m.UniqueWindows()); got != 2 {
		t.Errorf("UniqueWindows() with distinct window: expected 2 unique windows, got %d", got)
	}
}

func TestNthWeekdaySchedule_Matches(t *testing.T) {
	sched1stMon := &NthWeekdaySchedule{Weekday: time.Monday, Occurrence: 1}
	sched3rdTue := &NthWeekdaySchedule{Weekday: time.Tuesday, Occurrence: 3}
	schedLastTue := &NthWeekdaySchedule{Weekday: time.Tuesday, Last: true}

	tests := []struct {
		desc  string
		sched *NthWeekdaySchedule
		date  time.Time
		want  bool
	}{
		{"weekday mismatch", sched3rdTue, time.Date(2025, time.April, 16, 12, 0, 0, 0, time.UTC), false},
		{"occurrence mismatch", sched3rdTue, time.Date(2025, time.April, 1, 12, 0, 0, 0, time.UTC), false},
		{"occurrence mismatch 2nd tue", sched3rdTue, time.Date(2025, time.April, 8, 12, 0, 0, 0, time.UTC), false},
		{"occurrence match", sched3rdTue, time.Date(2025, time.April, 15, 12, 0, 0, 0, time.UTC), true},
		{"occurrence mismatch 4th tue", sched3rdTue, time.Date(2025, time.April, 22, 12, 0, 0, 0, time.UTC), false},
		{"1st occurrence boundary day 7", sched1stMon, time.Date(2025, time.April, 7, 12, 0, 0, 0, time.UTC), true},
		{"1st occurrence mismatch day 14", sched1stMon, time.Date(2025, time.April, 14, 12, 0, 0, 0, time.UTC), false},
		{"last weekday mismatch", schedLastTue, time.Date(2025, time.April, 28, 12, 0, 0, 0, time.UTC), false},
		{"last not last occurrence", schedLastTue, time.Date(2025, time.April, 22, 12, 0, 0, 0, time.UTC), false},
		{"last occurrence match", schedLastTue, time.Date(2025, time.April, 29, 12, 0, 0, 0, time.UTC), true},
		{"last occurrence Dec year boundary", schedLastTue, time.Date(2025, time.December, 30, 12, 0, 0, 0, time.UTC), true},
		{"last occurrence Dec not last", schedLastTue, time.Date(2025, time.December, 23, 12, 0, 0, 0, time.UTC), false},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			if got := tc.sched.matches(tc.date); got != tc.want {
				t.Errorf("matches(%v): got %v, want %v", tc.date, got, tc.want)
			}
		})
	}
}

func TestNthWeekdaySchedule_NextWithDailyBase(t *testing.T) {
	base, err := cronParser.Parse("0 0 12 * * *")
	if err != nil {
		t.Fatalf("cronParser.Parse failed: %v", err)
	}
	sched := &NthWeekdaySchedule{Base: base, Weekday: time.Tuesday, Occurrence: 2}
	from := time.Date(2025, time.April, 1, 0, 0, 0, 0, time.Local)
	want := time.Date(2025, time.April, 8, 12, 0, 0, 0, time.Local)
	if got := sched.Next(from); !got.Equal(want) {
		t.Errorf("sched.Next(%v) with daily base: got %v, want %v", from, got, want)
	}
	// Verify rollover to next month when starting after occurrence.
	fromAfter := time.Date(2025, time.April, 9, 0, 0, 0, 0, time.Local)
	wantMay := time.Date(2025, time.May, 13, 12, 0, 0, 0, time.Local)
	if got := sched.Next(fromAfter); !got.Equal(wantMay) {
		t.Errorf("sched.Next(%v) with daily base after occurrence: got %v, want %v", fromAfter, got, wantMay)
	}
}

type mockSchedule struct {
	nextFunc func(time.Time) time.Time
}

func (m *mockSchedule) Next(t time.Time) time.Time {
	return m.nextFunc(t)
}

func TestNthWeekdaySchedule_NextYearLimit(t *testing.T) {
	from := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	targetYear5 := time.Date(2030, time.January, 1, 12, 0, 0, 0, time.UTC) // Jan 1 2030 is 1st Tue (t.Year()+5)
	targetYear6 := time.Date(2031, time.January, 7, 12, 0, 0, 0, time.UTC) // Jan 7 2031 is 1st Tue (t.Year()+6)

	targetMock := func(target time.Time) cron.Schedule {
		return &mockSchedule{nextFunc: func(t time.Time) time.Time {
			if t.Before(target) {
				return target
			}
			return time.Time{}
		}}
	}

	tests := []struct {
		desc string
		base cron.Schedule
		want time.Time
	}{
		{"zero base returns zero", &mockSchedule{nextFunc: func(time.Time) time.Time { return time.Time{} }}, time.Time{}},
		{"within year limit (t.Year()+5)", targetMock(targetYear5), targetYear5},
		{"exceeds year limit (t.Year()+6)", targetMock(targetYear6), time.Time{}},
		{"no match loop exceeding limit", &mockSchedule{nextFunc: func(t time.Time) time.Time {
			next := t.AddDate(1, 0, 0)
			for next.Weekday() != time.Wednesday {
				next = next.AddDate(0, 0, 1)
			}
			return next
		}}, time.Time{}},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			sched := &NthWeekdaySchedule{Base: tc.base, Weekday: time.Tuesday, Occurrence: 1}
			got := sched.Next(from)
			if tc.want.IsZero() {
				if !got.IsZero() {
					t.Errorf("Next(): got %v, want zero time", got)
				}
			} else if !got.Equal(tc.want) {
				t.Errorf("Next(): got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNthWeekdaySchedule_Equal(t *testing.T) {
	base1, err := cronParser.Parse("0 0 17 * * tue")
	if err != nil {
		t.Fatalf("cronParser.Parse failed: %v", err)
	}
	base2, err := cronParser.Parse("0 0 18 * * wed")
	if err != nil {
		t.Fatalf("cronParser.Parse failed: %v", err)
	}

	s := &NthWeekdaySchedule{Base: base1, Weekday: time.Tuesday, Occurrence: 3}
	sLast := &NthWeekdaySchedule{Base: base1, Weekday: time.Tuesday, Last: true}

	tests := []struct {
		desc string
		a, b *NthWeekdaySchedule
		want bool
	}{
		{"nil == nil", nil, nil, true},
		{"s != nil", s, nil, false},
		{"nil != s", nil, s, false},
		{"identical", s, &NthWeekdaySchedule{Base: base1, Weekday: time.Tuesday, Occurrence: 3}, true},
		{"identical Last", sLast, &NthWeekdaySchedule{Base: base1, Weekday: time.Tuesday, Last: true}, true},
		{"diff Weekday", s, &NthWeekdaySchedule{Base: base1, Weekday: time.Wednesday, Occurrence: 3}, false},
		{"diff Occurrence", s, &NthWeekdaySchedule{Base: base1, Weekday: time.Tuesday, Occurrence: 4}, false},
		{"diff Last", s, &NthWeekdaySchedule{Base: base1, Weekday: time.Tuesday, Occurrence: 3, Last: true}, false},
		{"diff Base", s, &NthWeekdaySchedule{Base: base2, Weekday: time.Tuesday, Occurrence: 3}, false},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			if got := tc.a.Equal(tc.b); got != tc.want {
				t.Errorf("a.Equal(b): got %v, want %v", got, tc.want)
			}
			if got := tc.b.Equal(tc.a); got != tc.want {
				t.Errorf("b.Equal(a): got %v, want %v", got, tc.want)
			}
			if got := cmp.Equal(tc.a, tc.b); got != tc.want {
				t.Errorf("cmp.Equal(a, b): got %v, want %v", got, tc.want)
			}
			if got := cmp.Equal(tc.b, tc.a); got != tc.want {
				t.Errorf("cmp.Equal(b, a): got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseSchedule_TrimSpace(t *testing.T) {
	specs := []struct {
		desc     string
		spec     string
		wantDOW  time.Weekday
		wantOcc  int
		wantLast bool
		isNth    bool
	}{
		{"6-field hash schedule with leading/trailing whitespace and CRON_TZ", "   CRON_TZ=UTC 0 0 17 * * Tue#3   ", time.Tuesday, 3, false, true},
		{"5-field hash schedule with leading/trailing whitespace and CRON_TZ", "   CRON_TZ=UTC 0 17 * * Tue#3   ", time.Tuesday, 3, false, true},
		{"6-field hash schedule with leading/trailing whitespace and TZ", "   TZ=America/New_York 0 0 17 * * Wed#1   ", time.Wednesday, 1, false, true},
		{"6-field hash schedule with leading/trailing whitespace and TZ=UTC", "   TZ=UTC 0 0 17 * * Wed#1   ", time.Wednesday, 1, false, true},
		{"6-field hash schedule with leading/trailing whitespace without TZ", "   0 0 17 * * Fri#L   ", time.Friday, 0, true, true},
		{"standard cron with leading/trailing whitespace and CRON_TZ", "   CRON_TZ=UTC 0 0 17 * * *   ", 0, 0, false, false},
		{"standard cron with leading/trailing whitespace without TZ", "   0 0 17 * * *   ", 0, 0, false, false},
	}

	for _, tc := range specs {
		t.Run(tc.desc, func(t *testing.T) {
			sched, err := parseSchedule(tc.spec)
			if err != nil {
				t.Fatalf("parseSchedule(%q) unexpected error: %v", tc.spec, err)
			}
			if tc.isNth {
				nth, ok := sched.(*NthWeekdaySchedule)
				if !ok {
					t.Fatalf("parseSchedule(%q) returned %T, want *NthWeekdaySchedule", tc.spec, sched)
				}
				if nth.Weekday != tc.wantDOW || nth.Occurrence != tc.wantOcc || nth.Last != tc.wantLast {
					t.Errorf("parseSchedule(%q): got (%v, %d, %v), want (%v, %d, %v)", tc.spec, nth.Weekday, nth.Occurrence, nth.Last, tc.wantDOW, tc.wantOcc, tc.wantLast)
				}
			}
			if next := sched.Next(time.Date(2025, time.April, 1, 0, 0, 0, 0, time.UTC)); next.IsZero() {
				t.Errorf("parseSchedule(%q) Next returned zero time", tc.spec)
			}
		})
	}
}
