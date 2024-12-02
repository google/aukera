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

// Package window provides window configuration retrieval and formatting capability.
package window

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/cabbie/metrics"
	"github.com/google/deck"
	"github.com/google/aukera/auklib"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/robfig/cron/v3"
)

// Format defines enum type for schedule formats.
type Format int16

const (
	// FormatCron denotes integer value for a crontab schedule expression.
	FormatCron Format = iota + 1
)

var cronParser = cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.DowOptional | cron.Descriptor)

// Map correlates windows to their defined labels.
type Map map[string][]Window

// UnmarshalJSON is a custom window Map unmarshaler.
func (m Map) UnmarshalJSON(b []byte) error {
	if bytes.Compare(b, []byte("null")) == 0 {
		return nil
	}
	s := struct {
		Windows []Window
	}{}
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	m.Add(s.Windows...)
	return nil
}

// MarshalJSON marshals Window Map to configuration JSON.
func (m Map) MarshalJSON() ([]byte, error) {
	jsonArr := struct {
		Windows []Window
	}{}
	jsonArr.Windows = append(jsonArr.Windows, m.UniqueWindows()...)
	return json.Marshal(jsonArr)
}

// Keys returns all configured label names.
func (m Map) Keys() []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Add adds windows to the appropriate label element(s).
func (m Map) Add(windows ...Window) {
	for _, w := range windows {
		for _, l := range w.Labels {
			m[l] = append(m[l], w)
		}
	}
}

// Find returns a Window slice that have the passed label.
func (m Map) Find(l string) []Window {
	return m[strings.ToLower(l)]
}

// FindWindow returns a Window with a given name from a slice
// of windows organized by label.
func (m Map) FindWindow(window, label string) Window {
	windows := m.Find(label)
	for _, w := range windows {
		if w.Name == window {
			return w
		}
	}
	return Window{}
}

// UniqueWindows returns all distinct windows stored in the Map.
func (m Map) UniqueWindows() []Window {
	var mapWindows []Window
	// Flatten Map.
	for _, k := range m.Keys() {
		mapWindows = append(mapWindows, m.Find(k)...)
	}
	// window contents evaluation function.
	contains := func(s []Window, w Window) bool {
		for i := range s {
			if cmp.Equal(s[i], w, cmpopts.IgnoreFields(cron.SpecSchedule{}, "Location")) {
				return true
			}
		}
		return false
	}
	var windows []Window
	// Only return unique windows.
	for _, w := range mapWindows {
		if !contains(windows, w) {
			windows = append(windows, w)
		}
	}
	return windows
}

func dedupSchedules(schedules []Schedule) []Schedule {
	var unique []Schedule
	keys := make(map[Schedule]bool)
	for _, s := range schedules {
		if !keys[s] {
			keys[s] = true
			unique = append(unique, s)
		}
	}
	return unique
}

// AggregateSchedules combines the schedules of labels that match a given string with those that overlap.
//
// This has the potential to return two or more schedules that that do not overlap. Schedule state happens
// within Aukera's schedule package.
func (m Map) AggregateSchedules(request string) []Schedule {
	request = strings.ToLower(request)
	var out, schedules []Schedule
	for _, w := range m[request] {
		sch := w.Schedule // dereference window schedule to set label as schedule name
		sch.Name = request
		schedules = append(schedules, sch)
	}
	sort.Slice(schedules, func(i int, j int) bool { return schedules[i].Opens.Before(schedules[j].Opens) })

	for len(schedules) > 0 {
		l := schedules[0]
		schedules = schedules[1:]
		for i := len(schedules) - 1; i >= 0; i-- {
			if err := l.Combine(schedules[i]); err != nil {
				continue
			}
			schedules = append(schedules[:i], schedules[i+1:]...)
		}
		out = append(out, l)
	}
	return dedupSchedules(out)
}

// Window for holding raw window JSON data.
type Window struct {
	Name, CronString string
	Format           Format
	Cron             cron.Schedule
	Duration         time.Duration
	Starts, Expires  time.Time
	Labels           []string
	Schedule         Schedule
}

type windowJSON struct {
	Name, Schedule, Duration string
	Starts, Expires          time.Time
	Format                   Format
	Labels                   []string
}

// UnmarshalJSON is a custom Window unmarshaler.
func (w *Window) UnmarshalJSON(b []byte) error {
	if bytes.Compare(b, []byte("null")) == 0 {
		return nil
	}

	var conv windowJSON
	if err := json.Unmarshal(b, &conv); err != nil {
		return err
	}

	if conv.Name == "" {
		return fmt.Errorf("window name not defined")
	}
	w.Name = conv.Name

	var err error
	switch conv.Format {
	case FormatCron:
		w.Cron, err = cronParser.Parse(conv.Schedule)
		if err != nil {
			return fmt.Errorf("window(%s): error processing schedule %q: %v", w.Name, conv.Schedule, err)
		}
	default:
		return fmt.Errorf("window(%s): invalid format specified: %d", w.Name, conv.Format)
	}
	w.Format = conv.Format

	if len(conv.Labels) == 0 {
		return fmt.Errorf("window(%s): window must have minimum of one label (found: %d)", w.Name, len(conv.Labels))
	}
	w.Labels = auklib.UniqueStrings(conv.Labels)

	w.Starts = conv.Starts
	w.Expires = conv.Expires
	w.CronString = conv.Schedule

	w.Duration, err = time.ParseDuration(conv.Duration)
	if err != nil {
		return err
	}
	w.calculateSchedule()

	return nil
}

// MarshalJSON is a custom marshaler for Window to ensure JSON output
// matches the fields within its configuration file.
func (w Window) MarshalJSON() ([]byte, error) {
	return json.Marshal(windowJSON{
		Name:     w.Name,
		Schedule: w.CronString,
		Duration: w.Duration.String(),
		Starts:   w.Starts,
		Expires:  w.Expires,
		Format:   w.Format,
		Labels:   w.Labels,
	})
}

// Expired determines window validity comparing Expiration time to time.Now().
func (w *Window) Expired() bool {
	if w.Expires.IsZero() {
		return false
	}
	return w.Expires.Before(time.Now())
}

// Started determines window validity comparing Started time to time.Now().
func (w *Window) Started() bool {
	return w.Starts.Before(time.Now())
}

func (w *Window) calculateSchedule() {
	type activation struct {
		open, close time.Time
	}
	var last, next activation
	now := time.Now()
	switch {
	case w.Started() && !w.Expired():
		last.open = w.LastActivation(now)
		next.open = w.NextActivation(now)
	case w.Expired():
		last.open = w.LastActivation(w.Expires)
		// Set Next.open to be the last activation of last.open when the
		// window has expired in order to represent the last valid window.
		next.open = w.LastActivation(last.open)
	case !w.Started():
		last.open = w.NextActivation(w.Starts)
		next.open = last.open
	}
	last.close = last.open.Add(w.Duration)
	next.close = next.open.Add(w.Duration)
	if last.open.Before(now) && now.Before(last.close) {
		w.Schedule.Opens = last.open.Local()
		w.Schedule.Closes = last.close.Local()
	} else {
		w.Schedule.Opens = next.open.Local()
		w.Schedule.Closes = next.close.Local()
	}

	if w.Schedule.IsOpen() {
		w.Schedule.State = "open"
	} else {
		w.Schedule.State = "closed"
	}

	w.Schedule.Duration = w.Duration
}

// NextActivation determines the next activation time of cron.Schedule.
// This function crawls back in time search last and current time values
// for match, solving case where each second within the cron string itself is a valid
// "Next" value.
func (w *Window) NextActivation(ts time.Time) time.Time {
	start := time.Now()
	// Schedules in the seconds are not supported. Adjusting passed timestamp
	// to the "floor" of the given minute.
	ts = ts.Add(-time.Duration(ts.Second()) * time.Second)

	cr, err := cronParser.Parse("* * * * * *")
	if err != nil {
		deck.Warningf("NextActivation: error parsing open cron string")
	}
	// An open cron string (activates every minute) will never reach a quorum
	// between two values. Return given time after seconds are removed.
	if w.Format == FormatCron && cmp.Equal(w.Cron, cr, cmpopts.IgnoreFields(cron.SpecSchedule{}, "Location")) {
		return ts
	}
	a := w.Cron.Next(ts)
	// Activation time search timeout
	for time.Since(start) < (5 * time.Second) {
		b := w.Cron.Next(a.Add(-2 * time.Second))
		if a.Equal(b) {
			return b
		}
		a = b
	}
	return time.Time{}
}

// LastActivation determines the last activation time of cron.Schedule.
// Cron itself is unaware of the duration of the window and states the window is closed
// if the defined cron is in the past. LastActivation travels back in time equal to the
// duration between now and the "Next" activation to find the starting timestamp of the
// last window.
func (w *Window) LastActivation(date time.Time) time.Time {
	var (
		next = w.NextActivation(date)
		last = next
	)
	// Incrementing with Fibonacci numbers as its ramp is most likely to
	// catch schedules of all frequencies. Omitting the first number in
	// sequence (0) as it provides no value, only computational cost.
	fibCurrent, fibLast := 1, 1
	for next.Equal(last) {
		fibCurrent, fibLast = fibLast, fibCurrent+fibLast
		last = w.NextActivation(date.Add(-time.Duration(fibCurrent) * time.Minute))
	}
	return last
}

// Schedule defines struct for schedule information.
type Schedule struct {
	Name, State   string
	Duration      time.Duration
	Opens, Closes time.Time
}

// MarshalJSON is a custom marshaler for Schedule to ensure the Duration
// value is marshalled as a human-readable string.
func (s *Schedule) MarshalJSON() ([]byte, error) {
	type temp Schedule
	return json.Marshal(&struct {
		*temp
		Duration string
	}{
		temp:     (*temp)(s),
		Duration: s.Duration.String(),
	},
	)
}

// UnmarshalJSON is a custom unmarshaller for Schedule struct. Used with
// client package to retrieve window schedule.
func (s *Schedule) UnmarshalJSON(b []byte) error {
	if bytes.Compare(b, []byte("null")) == 0 {
		return nil
	}

	var temp = struct {
		Name, State, Duration string
		Opens, Closes         time.Time
	}{}
	err := json.Unmarshal(b, &temp)
	if err != nil {
		return err
	}

	s.Duration, err = time.ParseDuration(temp.Duration)
	if err != nil {
		return err
	}

	s.Name = temp.Name
	s.State = temp.State
	s.Opens = temp.Opens
	s.Closes = temp.Closes

	return nil
}

// Overlaps evalutes if one schedule falls during another.
func (s *Schedule) Overlaps(c Schedule) bool {
	// c opens earlier than and closes within s
	if c.Opens.Before(s.Opens) && s.Opens.Before(c.Closes) {
		return true
	}
	// c closes later than and opens within s
	if s.Closes.Before(c.Closes) && c.Opens.Before(s.Closes) {
		return true
	}
	// c opens and closes within s
	if s.Opens.Before(c.Opens) && c.Closes.Before(s.Closes) {
		return true
	}
	// s opens and closes within c
	if c.Opens.Before(s.Opens) && s.Closes.Before(c.Closes) {
		return true
	}
	// s and c match
	if c.Opens.Equal(s.Opens) && c.Closes.Equal(s.Closes) {
		return true
	}
	return false
}

// Combine combines one schedule's timeframe with another.
func (s *Schedule) Combine(c Schedule) error {
	if s.Name != c.Name {
		return fmt.Errorf("names to not match: %q != %q", s.Name, c.Name)
	}
	if !s.Overlaps(c) {
		return fmt.Errorf("schedules do not overlap")
	}
	if c.Opens.Before(s.Opens) {
		s.Opens = c.Opens.Local()
	}
	if s.Closes.Before(c.Closes) {
		s.Closes = c.Closes.Local()
	}
	now := time.Now()
	if now.Before(s.Closes) && s.Opens.Before(now) {
		s.State = "open"
	} else {
		s.State = "closed"
	}

	s.Duration = s.Closes.Sub(s.Opens)

	return nil
}

// IsOpen determines if schedule is open based on open/close times.
func (s *Schedule) IsOpen() bool {
	now := time.Now()
	return s.Opens.Before(now) && now.Before(s.Closes)
}

func (s Schedule) String() string {
	return fmt.Sprintf("%s: IsOpen(%t) | Open/Close(%v/%v) | Duration(%v)",
		s.Name, s.IsOpen(), s.Opens, s.Closes, s.Duration)
}

// ConfigReader defines filesystem interactions for Window configurations.
type ConfigReader interface {
	PathExists(string) (bool, error)
	AbsPath(string) (string, error)
	JSONFiles(string) ([]os.DirEntry, error)
	JSONContent(string) ([]byte, error)
}

// Reader is the implementation of ConfigReader for the window package.
type Reader struct{}

// PathExists wraps auklib.PathExists for testing purposes specific to
// the window.Windows function.
//
// auklib.PathExists is used in other packages in Aukera that do not have
// need for a ConfigReader.
func (r Reader) PathExists(path string) (bool, error) {
	return auklib.PathExists(path)
}

// AbsPath converts a given path to an absolute path and evaluates
// its existence.
func (r Reader) AbsPath(path string) (string, error) {
	var err error
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		path, err = filepath.Abs(path)
		if err != nil {
			return path, fmt.Errorf("AbsPath: failed to determine absolute path: %v", err)
		}
	}
	exist, err := r.PathExists(path)
	if err != nil {
		return "", fmt.Errorf("AbsPath: error finding path %q: %v", path, err)
	}
	if !exist {
		return "", fmt.Errorf("AbsPath: doesn't exist: %q", path)
	}
	return path, nil
}

// JSONFiles returns all JSON files in a given directory.
func (r Reader) JSONFiles(path string) ([]os.DirEntry, error) {
	abs, err := r.AbsPath(path)
	if err != nil {
		return nil, fmt.Errorf("JSONFiles: error determining absolute path: %v", err)
	}
	fi, err := os.ReadDir(abs)
	if err != nil {
		return nil, fmt.Errorf("JSONFiles: failed to enumerate files in %q: %v", abs, err)
	}
	var files []os.DirEntry
	for _, f := range fi {
		if strings.ToLower(filepath.Ext(f.Name())) != ".json" {
			continue
		}
		files = append(files, f)
	}
	return files, nil
}

// JSONContent returns the contents of JSON files.
func (r Reader) JSONContent(path string) ([]byte, error) {
	abs, err := r.AbsPath(path)
	if err != nil {
		return nil, fmt.Errorf("JSONContent: error determining absolute path: %v", err)
	}
	if strings.ToLower(filepath.Ext(abs)) != ".json" {
		return nil, fmt.Errorf("JSONContent: file is not JSON")
	}
	return os.ReadFile(abs)
}

// Windows gets all defined windows within given directory.
func Windows(dir string, cr ConfigReader) (Map, error) {
	files, err := cr.JSONFiles(dir)
	if err != nil {
		return nil, err
	}
	var windows []Window
	for _, f := range files {
		s := struct {
			Windows []Window
		}{}
		fp := filepath.Join(dir, f.Name())
		b, err := cr.JSONContent(fp)
		if err != nil {
			deck.Errorf("error reading file %q: %v", f.Name(), err)
			reportConfFileMetric(fp, "read_err")
			continue
		}
		if err := json.Unmarshal(b, &s); err != nil {
			deck.Errorf("UnmarshalJSON error: file %q: %v", f.Name(), err)
			reportConfFileMetric(fp, "unmarshal_err")
			continue
		}
		reportConfFileMetric(fp, "ok")
		windows = append(windows, s.Windows...)
	}
	m := make(Map)
	m.Add(windows...)
	return m, nil
}

func reportConfFileMetric(path, result string) {
	m, err := metrics.NewString(fmt.Sprintf("%s/%s", auklib.MetricRoot, "config_loader"), auklib.MetricSvc)
	if err != nil {
		deck.Warningf("could not create metric: %v", err)
		return
	}
	m.Data.AddStringField("file_path", path)
	m.Set(result)
}

// ActiveHoursWindow retrieves the built-in Active Hours maintenance windows if available.
func ActiveHoursWindow() (*Window, error) {
	activeStartTime, activeEndTime, err := auklib.ActiveHours()
	if err != nil {
		return nil, err
	}
	activeWindow := Window{
		Name:     "active_hours",
		Labels:   []string{"active_hours"},
		Starts:   activeStartTime,
		Expires:  activeEndTime,
		Duration: activeEndTime.Sub(activeStartTime),
		Schedule: Schedule{
			Name:     "active_hours",
			Opens:    activeStartTime,
			Closes:   activeEndTime,
			Duration: activeEndTime.Sub(activeStartTime),
		},
	}
	if activeWindow.Schedule.IsOpen() {
		activeWindow.Schedule.State = "open"
	} else {
		activeWindow.Schedule.State = "closed"
	}
	return &activeWindow, nil
}
