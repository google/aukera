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

// Aukera provides a local http interface for querying locally-defined maintenance windows.
package main

import (
	"os"

	"flag"
	"github.com/google/deck/backends/logger"
	"github.com/google/deck"
	"github.com/google/aukera/auklib"
)

var (
	runInDebug = flag.Bool("debug", false, "Run in debug mode")
	port       = flag.Int("port", auklib.ServicePort, "Define listening port")
)

func main() {
	// Initialize configuration directory
	exist, err := auklib.PathExists(auklib.ConfDir)
	if err != nil {
		deck.Errorf("unexpected error finding path %s: %v", auklib.ConfDir, err)
	}
	if exist == false {
		deck.Warning("Configuration directory does not exist. Attempting creation.")
		if err := os.MkdirAll(auklib.ConfDir, 0664); err != nil {
			deck.Warningf("Unable to create configuration directory: %v", err)
		}
	}

	// Initialize logger
	lf, err := os.OpenFile(auklib.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0664)
	if err != nil {
		deck.Fatalln("Failed to open log file: ", err)
		os.Exit(1)
	}
	defer lf.Close()
	deck.Add(logger.Init(lf, 0))
	defer deck.Close()

	if err := setup(); err != nil {
		deck.Fatalln("Setup exited with error: ", err)
		os.Exit(1)
	}

	err = run()
	if err != nil {
		deck.Fatalln("Run exited with error: ", err)
		os.Exit(1)
	}
}
